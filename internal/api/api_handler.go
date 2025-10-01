package apihandler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	db "performance-dashboard-backend/internal/database"
	collectionmodels "performance-dashboard-backend/internal/database/collection_models"
	"time"
)

// CORS middleware
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type Response struct {
	Message string `json:"message"`
}

type SessionData struct {
	Token    string
	TeamRole []*db.TeamRole
	Email    string
}

var sessions = map[string]SessionData{}

func PostHandlerPerformancePoint(w http.ResponseWriter, r *http.Request) {

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	isTeamStr := r.URL.Query().Get("isTeam")
	startTimeStr := body["startDate"].(string)
	endTimeStr := body["endDate"].(string)
	identifiersInterface := body["identifiers"].([]interface{})
	identifiers := make([]string, len(identifiersInterface))
	for i, v := range identifiersInterface {
		identifiers[i] = v.(string)
	}
	startTime, _ := time.Parse(time.RFC3339, startTimeStr)
	endTime, _ := time.Parse(time.RFC3339, endTimeStr)

	var results []*db.PerformancePoint
	for _, id := range identifiers {
		res, err := db.GetPerformancePoint(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), id, startTime, endTime, isTeamStr == "true")
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, res...)
	}
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(results)
}

func PostHandlerStaffMember(w http.ResponseWriter, r *http.Request) {

	teamRoles, ok := GetUserRole(r.Header.Get("Authorization"))
	if !ok || teamRoles == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// if contains Admin role, allow all teams
	isAdmin := false
	for _, role := range teamRoles {
		if role.Role == "Admin" {
			isAdmin = true
			break
		}
	}

	var managerOfTeams []string
	if !isAdmin {
		for _, role := range teamRoles {
			if role.Role == "Manager" {
				managerOfTeams = append(managerOfTeams, role.Team)
			}
		}
	}

	email, ok := GetEmailFromToken(r.Header.Get("Authorization"))
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	teamsStrs := body["teams"].([]interface{})

	var results []*collectionmodels.Member

	if len(teamsStrs) == 0 && isAdmin {
		// If no teams are specified, return all members
		res, err := db.GetMembersByTeam(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), "")
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, res...)
	} else {
		teams := make([]string, len(teamsStrs))

		if len(teamsStrs) == 0 && len(managerOfTeams) > 0 {
			teams = managerOfTeams
		}
		for _, t := range teamRoles {
			if !contains(teams, t.Team) {
				teams = append(teams, t.Team)
			}
		}

		for _, team := range teams {
			res, err := db.GetMembersByTeam(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), team)
			if err != nil {
				http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			results = append(results, res...)
		}

		if len(results) > 0 {
			if !isAdmin && email != "" {
				// if you are manager of a specific team, return all members in your teams
				// if you are not manager of that team, return only your own info
				var filteredResults []*collectionmodels.Member
				for _, member := range results {
					if contains(managerOfTeams, member.Team) || member.Email == email {
						filteredResults = append(filteredResults, member)
					}
				}
				results = filteredResults
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	body := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	email := body["email"]

	isInDatabase, err := db.IsEmailInDatabase(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), email)

	if err == nil && isInDatabase {

		teamRoles, _ := db.GetMemberRoles(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), email)
		// Set session data
		// use bcrypt
		hash := sha256.New()
		hash.Write([]byte(email + time.Now().String()))
		token := hex.EncodeToString(hash.Sum(nil))
		sessions[token] = SessionData{
			Token:    token,
			TeamRole: teamRoles,
			Email:    email,
		}

		// Return token
		w.Header().Set("Content-Type", "application/json")
		// parse this correctly to json format
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	} else {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
	}
}

func GetUserRole(token string) ([]*db.TeamRole, bool) {
	session, exists := sessions[token]
	if !exists {
		return nil, false
	}
	return session.TeamRole, true
}

func ClearSessionMapSchedule() {
	for {
		time.Sleep(24 * time.Hour)
		sessions = map[string]SessionData{}
	}
}

func GetEmailFromToken(token string) (string, bool) {
	session, exists := sessions[token]
	if !exists {
		return "", false
	}
	return session.Email, true
}

func HandleLastWeekTeamPerformance(w http.ResponseWriter, r *http.Request) {
	teamRoles, ok := GetUserRole(r.Header.Get("Authorization"))
	if !ok || teamRoles == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Println("Unauthorized access attempt")
		return
	}
	teams := []string{}
	for _, role := range teamRoles {
		if !contains(teams, role.Team) {
			teams = append(teams, role.Team)
		}
	}

	isAdmin := false
	for _, role := range teamRoles {
		if role.Role == "Admin" {
			isAdmin = true
			break
		}
	}

	if isAdmin {
		var err error
		teams, err = db.GetAllTeams(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"))
		if err != nil {
			log.Println("Error getting all teams:", err)
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	thisMonday := time.Now().AddDate(0, 0, -int(time.Now().Weekday())+1)
	lastMonday := thisMonday.AddDate(0, 0, -7)
	var results []*db.PerformancePoint
	if len(teams) > 0 {
		for _, team := range teams {
			res, err := db.GetPerformancePoint(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), team, lastMonday, thisMonday, true)
			if err != nil {
				http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
				log.Println("Database error:", err)
				return
			}
			results = append(results, res...)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func HandleTeamWeeklyTarget(w http.ResponseWriter, r *http.Request) {
	teamRoles, ok := GetUserRole(r.Header.Get("Authorization"))
	if !ok || teamRoles == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Println("Unauthorized access attempt")
		return
	}

	teams := []string{}
	for _, role := range teamRoles {
		if !contains(teams, role.Team) {
			teams = append(teams, role.Team)
		}
	}

	isAdmin := false
	for _, role := range teamRoles {
		if role.Role == "Admin" {
			isAdmin = true
			break
		}
	}

	if isAdmin {
		var err error
		teams, err = db.GetAllTeams(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"))
		if err != nil {
			log.Println("Error getting all teams:", err)
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var results []*db.TeamWeeklyTarget
	if len(teams) > 0 {
		for _, team := range teams {
			res, err := db.GetTeamWeeklyTarget(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_TEAM_WEEKLY_TARGET"), team)
			if err != nil {
				http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
				log.Println("Database error:", err)
				return
			}
			results = append(results, res)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

/// ======================================================
/// ============= Team Members Handler ===================

func HandleAddNewTeamMember(w http.ResponseWriter, r *http.Request) {

	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	member := &collectionmodels.Member{
		MemberID: body["MemberID"].(string),
		Name:     body["Name"].(string),
		YOB:      int(body["YOB"].(float64)),
		Email:    body["Email"].(string),
		Role:     body["Role"].(string),
		Team:     body["Team"].(string),
	}

	log.Println("Adding new member:", member)

	err := collectionmodels.InsertMemberToDataBase(db.GetMongoClient(), os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), member)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Member added successfully"}`))
}

func HandleGetAllTeamMembers(w http.ResponseWriter, r *http.Request) {

	// TOOD : implement role-based access control

	res, err := collectionmodels.GetAllMembers(db.GetMongoClient(), os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"))
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func HandleUpdateTeamMember(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	member := &collectionmodels.Member{
		MemberID: body["MemberID"].(string),
		Name:     body["Name"].(string),
		YOB:      int(body["YOB"].(float64)),
		Email:    body["Email"].(string),
		Role:     body["Role"].(string),
		Team:     body["Team"].(string),
	}

	err := collectionmodels.UpdateMemberToDataBase(db.GetMongoClient(), os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), member)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Member updated successfully"}`))
}

func HandleDeleteTeamMember(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	memberID := body["MemberID"].(string)
	log.Println("Deleting member with ID:", memberID)

	err := collectionmodels.DeleteMemberInDataBase(db.GetMongoClient(), os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), memberID)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Member deleted successfully"}`))
}

/// =========== End Team Members Handler ================
/// =====================================================

/// =====================================================
/// ============ Project Details Handler ================

func HandleAddNewProjectDetail(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	projectDetail := &collectionmodels.ProjectDetail{
		ProjectID: int(body["ProjectID"].(float64)),
		Project:   body["Project"].(string),
		Research:  body["Research"].(string),
		Art:       body["Art"].(string),
		Concept:   body["Concept"].(string),
		Video:     body["Video"].(string),
		Pla:       body["Pla"].(string),
		UA:        body["UA"].(string),
	}

	err := collectionmodels.InstertNewProjectDetailToDatabase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_PROJECT_DETAIL"), projectDetail)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Project detail added successfully"}`))
}

func HandleGetAllProjectDetails(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control

	res, err := collectionmodels.GetAllProjectDetails(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_PROJECT_DETAIL"))
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func HandleUpdateProjectDetail(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	projectDetail := &collectionmodels.ProjectDetail{
		ProjectID: int(body["ProjectID"].(float64)),
		Project:   body["Project"].(string),
		Research:  body["Research"].(string),
		Art:       body["Art"].(string),
		Concept:   body["Concept"].(string),
		Video:     body["Video"].(string),
		Pla:       body["Pla"].(string),
		UA:        body["UA"].(string),
	}
	err := collectionmodels.UpdateProjectDetailToDatabase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_PROJECT_DETAIL"), projectDetail)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Project detail updated successfully"}`))
}

func HandleDeleteProjectDetail(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	projectID := body["Project"].(string)
	err := collectionmodels.DeleteProjectDetailInDatabase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_PROJECT_DETAIL"), projectID)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Project detail deleted successfully"}`))
}

/// =========== End Project Detail Handler ================
/// =======================================================

/// =======================================================
/// =========== Creative Tool Handler =====================

func HandleGetAllCreativeTools(w http.ResponseWriter, r *http.Request) {

	// TOOD : implement role-based access control
	res, err := collectionmodels.GetAllCreativeTools(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_CREATIVE_TOOLS"))
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func HandleUpdateCreativeTool(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	pointsInterface := body["Point"].([]interface{})
	points := make([]float64, len(pointsInterface))
	for i, v := range pointsInterface {
		points[i] = v.(float64)
	}
	tool := &collectionmodels.CreativeTool{
		Team:     body["Team"].(string),
		ToolName: body["ToolName"].(string),
		Type:     body["Type"].(string),
		Point:    points,
	}
	err := collectionmodels.UpdateCreativeTool(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_CREATIVE_TOOLS"), tool)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Creative tool updated successfully"}`))
}

func HandleAddNewCreativeTool(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	pointsInterface := body["Point"].([]interface{})
	points := make([]float64, len(pointsInterface))
	for i, v := range pointsInterface {
		points[i] = v.(float64)
	}
	tool := &collectionmodels.CreativeTool{
		Team:     body["Team"].(string),
		ToolName: body["ToolName"].(string),
		Type:     body["Type"].(string),
		Point:    points,
	}
	err := collectionmodels.AddCreativeTool(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_CREATIVE_TOOLS"), tool)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Creative tool added successfully"}`))
}

func HandleDeleteCreativeTool(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	team := body["Team"].(string)
	toolName := body["ToolName"].(string)

	err := collectionmodels.DeleteCreativeTool(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_CREATIVE_TOOLS"), team, toolName)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Creative tool deleted successfully"}`))
}

/// =========== End Creative Tool Handler =================
/// =======================================================

// / =======================================================
// / ============ Level To Point Handler ===================

func HandleGetAllLevel(w http.ResponseWriter, r *http.Request) {

	// TOOD : implement role-based access control
	res, err := collectionmodels.GetAllLevels(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_LEVEL"))
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func HandleUpdateLevel(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	pointsInterface := body["Point"].([]interface{})
	points := make([]int, len(pointsInterface))
	for i, v := range pointsInterface {
		points[i] = int(v.(float64))
	}
	level := &collectionmodels.Level{
		Team:       body["Team"].(string),
		LevelPoint: points,
	}
	err := collectionmodels.UpdateLevelPointsForTeam(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_LEVEL"), level)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func HandleAddNewLevel(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	pointsInterface := body["Point"].([]interface{})
	points := make([]int, len(pointsInterface))
	for i, v := range pointsInterface {
		points[i] = int(v.(float64))
	}
	level := &collectionmodels.Level{
		Team:       body["Team"].(string),
		LevelPoint: points,
	}
	err := collectionmodels.AddNewLevelForTeam(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_LEVEL"), level)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "New level added successfully"}`))
}

func HandleDeleteLevel(w http.ResponseWriter, r *http.Request) {
	// TOOD : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	team := body["Team"].(string)

	err := collectionmodels.DeleteLevelForTeam(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_LEVEL"), team)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Level deleted successfully"}`))
}

// / =========== End Level To Point Handler ================
// / =======================================================

// / =======================================================
// / ============= Weekly Target Handler ===================

func HandleGetWeeklyTarget(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control
	target, err := collectionmodels.GetAllWeeklyTargets(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_WEEKLY_TARGET"))
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(target)
}

func HandleUpdateWeeklyTarget(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	team := body["Team"].(string)
	point := body["Point"].(float64)
	dateFromStr := body["DateFrom"].(string)
	dateToStr := body["DateTo"].(string)
	target := &collectionmodels.WeeklyTarget{
		Team:  team,
		Point: int(point),
		DateFrom: func() time.Time {
			t, _ := time.Parse(time.RFC3339, dateFromStr)
			return t
		}(),
		DateTo: func() time.Time {
			t, _ := time.Parse(time.RFC3339, dateToStr)
			return t
		}(),
	}
	err := collectionmodels.UpdateWeeklyTargetByTeam(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_WEEKLY_TARGET"), target)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Weekly target updated successfully"}`))
}

func HandleAddNewWeeklyTarget(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	team := body["Team"].(string)
	point := body["Point"].(float64)
	dateFromStr := body["DateFrom"].(string)
	dateToStr := body["DateTo"].(string)
	target := &collectionmodels.WeeklyTarget{
		Team:  team,
		Point: int(point),
		DateFrom: func() time.Time {
			t, _ := time.Parse(time.RFC3339, dateFromStr)
			return t
		}(),
		DateTo: func() time.Time {
			t, _ := time.Parse(time.RFC3339, dateToStr)
			return t
		}(),
	}
	err := collectionmodels.InsertWeeklyTarget(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_WEEKLY_TARGET"), target)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "New weekly target added successfully"}`))
}

func HandleDeleteWeeklyTarget(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	team := body["Team"].(string)
	dateFromStr := body["DateFrom"].(string)
	dateToStr := body["DateTo"].(string)
	dateFrom, _ := time.Parse(time.RFC3339, dateFromStr)
	dateTo, _ := time.Parse(time.RFC3339, dateToStr)
	err := collectionmodels.DeleteWeeklyTarget(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_WEEKLY_TARGET"), team, dateFrom, dateTo)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Weekly target deleted successfully"}`))
}

// / ============ End Weekly Target Handler =================
// / =======================================================

/// ============== Weekly Order Handler ===================

func HandleGetWeeklyOrder(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control

	res, err := collectionmodels.GetAllWeeklyOrders(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_WEEKLY_ORDER"))
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func HandleUpdateWeeklyOrder(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	startWeekStr := body["StartWeek"].(string)
	startWeek, _ := time.Parse(time.RFC3339, startWeekStr)

	order := &collectionmodels.WeeklyOrder{
		StartWeek: startWeek,
		Goal:      body["Goal"].(string),
		Strategy:  body["Strategy"].(string),
		Project:   body["Project"].(string),
		CPP:       body["CPP"].(int),
		Icon:      body["Icon"].(int),
		Banner:    body["Banner"].(int),
		Video:     body["Video"].(int),
		PLA:       body["PLA"].(int),
	}

	err := collectionmodels.UpdateWeeklyOrder(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_WEEKLY_ORDER"), order)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Weekly order updated successfully"}`))
}

func HandleAddNewWeeklyOrder(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	startWeekStr := body["StartWeek"].(string)
	startWeek, _ := time.Parse(time.RFC3339, startWeekStr)
	order := &collectionmodels.WeeklyOrder{
		StartWeek: startWeek,
		Goal:      body["Goal"].(string),
		Strategy:  body["Strategy"].(string),
		Project:   body["Project"].(string),
		CPP:       body["CPP"].(int),
		Icon:      body["Icon"].(int),
		Banner:    body["Banner"].(int),
		Video:     body["Video"].(int),
		PLA:       body["PLA"].(int),
	}
	err := collectionmodels.InsertWeeklyOrder(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_WEEKLY_ORDER"), order)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Weekly order added successfully"}`))
}

func HandleDeleteWeeklyOrder(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	startWeekStr := body["StartWeek"].(string)
	startWeek, _ := time.Parse(time.RFC3339, startWeekStr)
	project := body["Project"].(string)

	err := collectionmodels.DeleteWeeklyOrder(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_WEEKLY_ORDER"), startWeek, project)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Weekly order deleted successfully"}`))
}

/// =======================================================

/// ========================================================
/// =========== Project Issues Handler =====================

func HandlePostProjectIssues(w http.ResponseWriter, r *http.Request) {
	// TODO : implement role-based access control
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	startWeekStr := body["StartDate"].(string)
	startWeek, _ := time.Parse(time.RFC3339, startWeekStr)
	endWeekStr := body["EndDate"].(string)
	endWeek, _ := time.Parse(time.RFC3339, endWeekStr)

	issues, err := collectionmodels.GetProjectIssues(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_WEEKLY_ORDER"), startWeek, endWeek)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(issues)
}

/// =========== End Project Issues Handler =================
/// ========================================================

func Init() {
	http.Handle("/login", CORSMiddleware(http.HandlerFunc(LoginHandler)))

	http.Handle("/post/performance-point", CORSMiddleware(http.HandlerFunc(PostHandlerPerformancePoint)))
	http.Handle("/post/staff-member", CORSMiddleware(http.HandlerFunc(PostHandlerStaffMember)))
	http.Handle("/get/last-week-team-performance", CORSMiddleware(http.HandlerFunc(HandleLastWeekTeamPerformance)))
	http.Handle("/get/team-weekly-target", CORSMiddleware(http.HandlerFunc(HandleTeamWeeklyTarget)))

	http.Handle("/get/team-members", CORSMiddleware(http.HandlerFunc(HandleGetAllTeamMembers)))
	http.Handle("/post/update-team-member", CORSMiddleware(http.HandlerFunc(HandleUpdateTeamMember)))
	http.Handle("/post/add-new-team-member", CORSMiddleware(http.HandlerFunc(HandleAddNewTeamMember)))
	http.Handle("/post/delete-team-member", CORSMiddleware(http.HandlerFunc(HandleDeleteTeamMember)))

	http.Handle("/get/project-details", CORSMiddleware(http.HandlerFunc(HandleGetAllProjectDetails)))
	http.Handle("/post/add-new-project-detail", CORSMiddleware(http.HandlerFunc(HandleAddNewProjectDetail)))
	http.Handle("/post/update-project-detail", CORSMiddleware(http.HandlerFunc(HandleUpdateProjectDetail)))
	http.Handle("/post/delete-project-detail", CORSMiddleware(http.HandlerFunc(HandleDeleteProjectDetail)))

	http.Handle("/get/creative-tools", CORSMiddleware(http.HandlerFunc(HandleGetAllCreativeTools)))
	http.Handle("/post/update-creative-tool", CORSMiddleware(http.HandlerFunc(HandleUpdateCreativeTool)))
	http.Handle("/post/add-new-creative-tool", CORSMiddleware(http.HandlerFunc(HandleAddNewCreativeTool)))
	http.Handle("/post/delete-creative-tool", CORSMiddleware(http.HandlerFunc(HandleDeleteCreativeTool)))

	http.Handle("/get/levels", CORSMiddleware(http.HandlerFunc(HandleGetAllLevel)))
	http.Handle("/post/update-level", CORSMiddleware(http.HandlerFunc(HandleUpdateLevel)))
	http.Handle("/post/add-new-level", CORSMiddleware(http.HandlerFunc(HandleAddNewLevel)))
	http.Handle("/post/delete-level", CORSMiddleware(http.HandlerFunc(HandleDeleteLevel)))

	http.Handle("/get/weekly-target", CORSMiddleware(http.HandlerFunc(HandleGetWeeklyTarget)))
	http.Handle("/post/update-weekly-target", CORSMiddleware(http.HandlerFunc(HandleUpdateWeeklyTarget)))
	http.Handle("/post/add-new-weekly-target", CORSMiddleware(http.HandlerFunc(HandleAddNewWeeklyTarget)))
	http.Handle("/post/delete-weekly-target", CORSMiddleware(http.HandlerFunc(HandleDeleteWeeklyTarget)))

	http.Handle("/get/weekly-order", CORSMiddleware(http.HandlerFunc(HandleGetWeeklyOrder)))
	http.Handle("/post/update-weekly-order", CORSMiddleware(http.HandlerFunc(HandleUpdateWeeklyOrder)))
	http.Handle("/post/add-new-weekly-order", CORSMiddleware(http.HandlerFunc(HandleAddNewWeeklyOrder)))
	http.Handle("/post/delete-weekly-order", CORSMiddleware(http.HandlerFunc(HandleDeleteWeeklyOrder)))

	http.Handle("/post/project-issues", CORSMiddleware(http.HandlerFunc(HandlePostProjectIssues)))

	go ClearSessionMapSchedule()
}
