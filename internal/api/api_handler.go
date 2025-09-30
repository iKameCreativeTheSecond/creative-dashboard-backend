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

	log.Println("User roles:", teamRoles, "isAdmin:", isAdmin, "is Manager:", len(managerOfTeams) > 0)

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
		log.Println("Teams to query:", teams)

		for _, team := range teams {
			res, err := db.GetMembersByTeam(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), team)
			if err != nil {
				http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			results = append(results, res...)
		}

		log.Println("Results length:", len(results))

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

	log.Println("Fetching last week's performance for teams:", teams)

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
	log.Println("Returning team weekly targets for teams:", teams)
	for _, r := range results {
		// log.Printf("Team: %s, Point: %d, DateFrom: %s, DateTo: %s\n", r.Team, r.Point, r.DateFrom.Format("2006-01-02"), r.DateTo.Format("2006-01-02"))
		log.Printf("Team: %s, Point: %f\n", r.Team, r.WeeklyTarget)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func Init() {
	http.Handle("/login", CORSMiddleware(http.HandlerFunc(LoginHandler)))
	http.Handle("/post/performance-point", CORSMiddleware(http.HandlerFunc(PostHandlerPerformancePoint)))
	http.Handle("/post/staff-member", CORSMiddleware(http.HandlerFunc(PostHandlerStaffMember)))
	http.Handle("/get/last-week-team-performance", CORSMiddleware(http.HandlerFunc(HandleLastWeekTeamPerformance)))
	http.Handle("/get/team-weekly-target", CORSMiddleware(http.HandlerFunc(HandleTeamWeeklyTarget)))
	go ClearSessionMapSchedule()
}
