package apihandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	db "performance-dashboard-backend/internal/database"
	collectionmodels "performance-dashboard-backend/internal/database/collection_models"
	"time"

	"github.com/gorilla/sessions"
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

const USER_ID = "user_id"
const AUTH_SESSION = "auth-session"
const TEAM_ROLE = "team_role"

var sessionStore = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

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
	teamsStrs := body["teams"].([]interface{})

	var results []*collectionmodels.Member

	if len(teamsStrs) == 0 {
		// If no teams are specified, return all members
		res, err := db.GetMembersByTeam(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), "")
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, res...)
	} else {
		team := make([]string, len(teamsStrs))
		for i, v := range teamsStrs {
			team[i] = v.(string)
			res, err := db.GetMembersByTeam(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), team[i])
			if err != nil {
				http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			results = append(results, res...)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := r.FormValue("email")

	isInDatabase, err := db.IsEmailInDatabase(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), email)
	if err != nil && isInDatabase {

		teamRoles, _ := db.GetMemberRoles(os.Getenv("MONGO_URI"), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"), email)

		session, _ := sessionStore.Get(r, AUTH_SESSION)
		session.Values[USER_ID] = email
		session.Values[TEAM_ROLE] = teamRoles
		session.Save(r, w)

		w.Write([]byte("Login success"))
	} else {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
	}
}

func GetTeamRoles(r *http.Request) ([]db.TeamRole, error) {
	session, err := sessionStore.Get(r, AUTH_SESSION)
	if err != nil {
		return nil, err
	}
	teamRoles, ok := session.Values[TEAM_ROLE].([]db.TeamRole)
	if !ok {
		return nil, errors.New("err: invalid team roles")
	}
	return teamRoles, nil
}

func IsAuthenticated(r *http.Request) bool {
	session, err := sessionStore.Get(r, AUTH_SESSION)
	if err != nil {
		return false
	}
	_, ok := session.Values[USER_ID].(string)
	return ok
}

func Init() {
	http.Handle("/login", CORSMiddleware(http.HandlerFunc(LoginHandler)))
	http.Handle("/post/performance-point", CORSMiddleware(http.HandlerFunc(PostHandlerPerformancePoint)))
	http.Handle("/post/staff-member", CORSMiddleware(http.HandlerFunc(PostHandlerStaffMember)))
	// http.Handle("/", CORSMiddleware(http.HandlerFunc(RootHandler)))
	// http.Handle("/get", CORSMiddleware(http.HandlerFunc(GetHandler)))
	// http.Handle("/post", CORSMiddleware(http.HandlerFunc(PostHandler)))
}
