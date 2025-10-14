package main

import (
	"log"
	"os"
	"performance-dashboard-backend/internal/asana"
	db "performance-dashboard-backend/internal/database"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func ConnectDatabase() {
	err := db.ConnectMongoDB()
	if err != nil {
		log.Fatal("Database connection error:", err)
	}
}

func main() {
	LoadEnv()
	// ConnectDatabase()

	// FetchAsanaTasks()
	asana.FetchAsanaTasksTeamPlayable("PLA", os.Getenv("ASANA_PROJECT_ID_PLA"))

	// api.Init()
	// log.Fatal(http.ListenAndServe(":"+os.Getenv("SERVER_PORT"), nil))
}
