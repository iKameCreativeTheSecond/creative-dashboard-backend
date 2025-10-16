package main

import (
	"log"
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
	ConnectDatabase()

	// asana.SyncronizeWeeklyTasks()
	// api.Init()
	// log.Fatal(http.ListenAndServe(":"+os.Getenv("SERVER_PORT"), nil))
}
