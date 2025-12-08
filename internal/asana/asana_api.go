package asana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	collectionmodels "performance-dashboard-backend/internal/database/collection_models"
	"regexp"
	"strconv"
	"time"

	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/mongo"

	db "performance-dashboard-backend/internal/database"
)

func FetchTasks(token string, projectID string) ([]Task, error) {
	var allTasks []Task
	url := fmt.Sprintf("https://app.asana.com/api/1.0/projects/%s/tasks?opt_fields=name,assignee.name,assignee.email,completed,due_on,custom_fields&limit=50", projectID)
	for {
		// Build request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		// Send request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		// Read body
		body, _ := io.ReadAll(resp.Body)

		// Parse JSON
		var asanaResp AsanaResponse
		if err := json.Unmarshal(body, &asanaResp); err != nil {
			fmt.Printf("Error unmarshalling Asana response: %v\nResponse body: %s\n", err, string(body))
			return nil, err
		}

		// Collect tasks
		allTasks = append(allTasks, asanaResp.Data...)

		// Check pagination
		if asanaResp.NextPage == nil {
			break
		}
		url = asanaResp.NextPage.Uri
	}

	return allTasks, nil
}

func InsertCompletedTaskToDataBase(client *mongo.Client, dbName, collName string, task *collectionmodels.CompletedTask) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	_, err := collection.InsertOne(ctx, task)
	return err
}

func FetchAsanaTasksByTeam(team string, projectID string) []*collectionmodels.CompletedTask {
	token := os.Getenv("ASANA_TOKEN") // safer to set as env var
	tasks, err := FetchTasks(token, projectID)
	if err != nil {
		fmt.Println("Error:", err)
		return nil
	}

	var completedTasks []*collectionmodels.CompletedTask
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	// This Monday at 09:00 Vietnam time
	thisMondayAtNine := time.Date(
		now.Year(),
		now.Month(),
		now.Day()-weekday+1,
		9, 0, 0, 0,
		loc,
	)
	count := 0
	rejectedCount := 0
	for _, task := range tasks {

		if !task.Completed {

			continue
		}
		count++

		// Map Asana task to internal model
		var toolIndexes []int
		var level int
		var projectName string
		for _, field := range task.CustomFields {
			if field.Name == "Tool/CTST PLA" || field.Name == "Tool/CTST Video" || field.Name == "Tool/CTST Art" || field.Name == "Tool/CTST Concept" {
				toolIndexes = GetListToolAsIndexes(field.DisplayValue)
			}
			if field.Name == "PLA Difficult" || field.Name == "Art point" || field.Name == "Video Difficult" || field.Name == "Concept Difficult" {

				// fmt.Println("Parsing level from field:", field.Name, "with value:", field.DisplayValue)
				// Try parsing as integer first, fall back to float
				// parsed := false
				if lvl, err := strconv.Atoi(field.DisplayValue); err == nil {
					level = lvl
					// parsed = true
				} else if lvl, err := strconv.ParseFloat(field.DisplayValue, 64); err == nil {
					level = int(lvl)
					// parsed = true
				}
				// if !parsed {
				// 	fmt.Printf("Warning: could not parse level from field '%s' with value '%s'\n", field.Name, field.DisplayValue)
				// }
			}
			if field.Name == "Game Name" {
				projectName = field.DisplayValue
			}
		}
		// fmt.Println("Processing completed task:", task.Name, "Level:", level, "Tools:", toolIndexes)
		if level < 1 {
			rejectedCount++
			continue
		}

		if toolIndexes == nil {
			toolIndexes = []int{}
		}

		completedTask := &collectionmodels.CompletedTask{
			TaskID:     task.Gid,
			DoneDate:   thisMondayAtNine,
			TaskName:   task.Name,
			AssigneeID: task.Assignee.Email,
			Team:       team,
			Tool:       toolIndexes,
			Project:    projectName,
			Level:      level,
		}
		// frint all the fields
		// fmt.Printf("TaskID: %s, TaskName: %s, AssigneeID: %s, Team: %s, Tool: %v, Level: %d, Project: %s, DoneDate: %s\n",
		// 	completedTask.TaskID, completedTask.TaskName, completedTask.AssigneeID, completedTask.Team,
		// 	completedTask.Tool, completedTask.Level, completedTask.Project, completedTask.DoneDate.Format("2006-01-02"))

		//fmt.Print("ID ", completedTask.TaskID, " | Name: ", completedTask.TaskName, " | Assignee: ", completedTask.AssigneeID, " | Tool: ", completedTask.Tool, " | Level: ", completedTask.Level, " | Project: ", completedTask.Project, " | DoneDate: ", completedTask.DoneDate.Format("2006-01-02"), "\n")
		completedTasks = append(completedTasks, completedTask)
	}
	return completedTasks
}

func GetListToolAsIndexes(s string) []int {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(s, -1)

	var numbers []int
	for _, m := range matches {
		n, err := strconv.Atoi(m)
		if err == nil {
			numbers = append(numbers, n)
		}
	}
	return numbers
}

// schedule to run every Monday at 11:59 AM
// Implementation of scheduling logic goes here
func ScheduleWeeklyTaskSync() {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		fmt.Println("Cannot load Asia/Ho_Chi_Minh, fallback UTC:", err)
		loc = time.UTC
	}
	c := cron.New(cron.WithLocation(loc))
	_, err = c.AddFunc("59 23 * * 1", SyncronizeWeeklyTasks)
	if err != nil {
		fmt.Println("Cron add error:", err)
		return
	}
	c.Start()
}

func SyncronizeWeeklyTasks() {
	fmt.Println("Starting weekly Asana task synchronization...")
	plaCompltedTasks := FetchAsanaTasksByTeam("PLA", os.Getenv("ASANA_PROJECT_ID_PLA"))
	fmt.Printf("Inserting %d PLA completed tasks into the database...\n", len(plaCompltedTasks))
	if len(plaCompltedTasks) > 0 {
		collectionmodels.InsertCompletedTaskToDataBase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), plaCompltedTasks)
	}
	videoCompletedTasks := FetchAsanaTasksByTeam("Video", os.Getenv("ASANA_PROJECT_ID_VIDEO"))
	fmt.Printf("Inserting %d Video completed tasks into the database...\n", len(videoCompletedTasks))
	if len(videoCompletedTasks) > 0 {
		collectionmodels.InsertCompletedTaskToDataBase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), videoCompletedTasks)
	}
	artCompletedTasks := FetchAsanaTasksByTeam("Art", os.Getenv("ASANA_PROJECT_ID_ART"))
	fmt.Printf("Inserting %d Art completed tasks into the database...\n", len(artCompletedTasks))
	if len(artCompletedTasks) > 0 {
		collectionmodels.InsertCompletedTaskToDataBase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), artCompletedTasks)
	}
	conceptCompletedTasks := FetchAsanaTasksByTeam("Concept", os.Getenv("ASANA_PROJECT_ID_CONCEPT"))
	fmt.Printf("Inserting %d Concept completed tasks into the database...\n", len(conceptCompletedTasks))
	if len(conceptCompletedTasks) > 0 {
		collectionmodels.InsertCompletedTaskToDataBase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), conceptCompletedTasks)
	}
	fmt.Println("Weekly Asana task synchronization completed.")
}
