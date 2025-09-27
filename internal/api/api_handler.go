package apihandler

import (
	"encoding/json"
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

// // Root handler
// func RootHandler(w http.ResponseWriter, r *http.Request) {
// 	res := Response{Message: "Welcome to Go API listener!"}
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(res)
// }

// // Example GET handler
// func GetHandler(w http.ResponseWriter, r *http.Request) {
// 	res := Response{Message: "This is a GET endpoint"}
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(res)
// }

// // Example POST handler
// func PostHandler(w http.ResponseWriter, r *http.Request) {
// 	var data map[string]interface{}
// 	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
// 		http.Error(w, "Invalid JSON", http.StatusBadRequest)
// 		return
// 	}

// 	res := map[string]interface{}{
// 		"received": data,
// 		"status":   "success",
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(res)
// }

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

func Init() {
	http.Handle("/post/performance-point", CORSMiddleware(http.HandlerFunc(PostHandlerPerformancePoint)))
	http.Handle("/post/staff-member", CORSMiddleware(http.HandlerFunc(PostHandlerStaffMember)))
	// http.Handle("/", CORSMiddleware(http.HandlerFunc(RootHandler)))
	// http.Handle("/get", CORSMiddleware(http.HandlerFunc(GetHandler)))
	// http.Handle("/post", CORSMiddleware(http.HandlerFunc(PostHandler)))
}
