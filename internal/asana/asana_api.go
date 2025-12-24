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
)

func FetchTaskList(token string, spaceID string) ([]ClickUpTaskListResponse, error) {
	// Build ClickUp API URL
	url := fmt.Sprintf("https://api.clickup.com/api/v2/space/%s/list", spaceID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read body
	body, _ := io.ReadAll(resp.Body)

	// Debug: Print response status and body
	fmt.Printf("ClickUp API Response Status: %d\n", resp.StatusCode)

	// Parse JSON - the response has a "lists" wrapper
	var listsResp ClickUpListsResponse
	if err := json.Unmarshal(body, &listsResp); err != nil {
		fmt.Printf("Error unmarshalling ClickUp response: %v\nResponse body: %s\n", err, string(body))
		return nil, err
	}

	return listsResp.Lists, nil
}

func FetchTasksFromAllLists(token string, spaceID string) ([]Task, error) {
	taskLists, err := FetchTaskList(token, spaceID)
	if err != nil {
		return nil, err
	}
	var allTasks []Task
	for _, list := range taskLists {
		tasks, err := FetchTasks(token, list.Id)
		if err != nil {
			return nil, err
		}
		allTasks = append(allTasks, tasks...)
	}
	return allTasks, nil
}

func FetchTasks(token string, listID string) ([]Task, error) {
	var allTasks []Task
	page := 0

	for {
		// Build ClickUp API URL with pagination
		url := fmt.Sprintf("https://api.clickup.com/api/v2/list/%s/task?page=%d", listID, page)

		// Build request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", token)
		req.Header.Set("Content-Type", "application/json")

		// Send request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		// Read body
		body, _ := io.ReadAll(resp.Body)

		// Debug: Print response status and body
		fmt.Printf("ClickUp API Response Status: %d\n", resp.StatusCode)
		fmt.Printf("ClickUp API Response Body: %s\n", string(body))

		// Parse JSON
		var clickupResp ClickUpResponse
		if err := json.Unmarshal(body, &clickupResp); err != nil {
			fmt.Printf("Error unmarshalling ClickUp response: %v\nResponse body: %s\n", err, string(body))
			return nil, err
		}

		// Collect tasks
		allTasks = append(allTasks, clickupResp.Tasks...)

		// Check if last page
		if clickupResp.LastPage || len(clickupResp.Tasks) == 0 {
			break
		}
		page++
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

func FetchAsanaTasksByTeam(team string, spaceID string) []*collectionmodels.CompletedTask {
	token := os.Getenv("CLICKUP_TOKEN") // safer to set as env var

	tasks, err := FetchTasksFromAllLists(token, spaceID)
	if err != nil {
		fmt.Println("Error:", err)
		return nil
	}

	fmt.Println("all task", len(tasks), "tasks fetched from ClickUp space", tasks)

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
		// Check if task is completed (closed status)
		if task.Status == nil || task.Status.Type != "closed" {
			continue
		}
		count++

		// Map ClickUp task to internal model
		var toolIndexes []int
		var level int
		var projectName string
		for _, field := range task.CustomFields {
			valueStr := fmt.Sprintf("%v", field.Value)

			if field.Name == "Tool/CTST PLA" || field.Name == "Tool/CTST Video" || field.Name == "Tool/CTST Art" || field.Name == "Tool/CTST Concept" {
				toolIndexes = GetListToolAsIndexes(valueStr)
			}
			if field.Name == "PLA Difficult" || field.Name == "Art point" || field.Name == "Video Difficult" || field.Name == "Concept Difficult" {
				// Try parsing as integer first, fall back to float
				if lvl, err := strconv.Atoi(valueStr); err == nil {
					level = lvl
				} else if lvl, err := strconv.ParseFloat(valueStr, 64); err == nil {
					level = int(lvl)
				}
			}
			if field.Name == "Game Name" {
				projectName = valueStr
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

		// Get assignee email (use first assignee if multiple)
		assigneeEmail := ""
		if len(task.Assignees) > 0 {
			assigneeEmail = task.Assignees[0].Email
		}

		completedTask := &collectionmodels.CompletedTask{
			TaskID:     task.Id,
			DoneDate:   thisMondayAtNine,
			TaskName:   task.Name,
			AssigneeID: assigneeEmail,
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
	fmt.Println("Starting weekly ClickUp task synchronization...")
	plaCompltedTasks := FetchAsanaTasksByTeam("PLA", os.Getenv("CLICKUP_SPACE_ID_CONCEPT"))
	fmt.Printf("Inserting %d PLA completed tasks into the database...\n", len(plaCompltedTasks))
	// if len(plaCompltedTasks) > 0 {
	// 	collectionmodels.InsertCompletedTaskToDataBase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), plaCompltedTasks)
	// }
	// videoCompletedTasks := FetchAsanaTasksByTeam("Video", os.Getenv("CLICKUP_SPACE_ID_VIDEO"))
	// fmt.Printf("Inserting %d Video completed tasks into the database...\n", len(videoCompletedTasks))
	// if len(videoCompletedTasks) > 0 {
	// 	collectionmodels.InsertCompletedTaskToDataBase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), videoCompletedTasks)
	// }
	// artCompletedTasks := FetchAsanaTasksByTeam("Art", os.Getenv("CLICKUP_SPACE_ID_ART"))
	// fmt.Printf("Inserting %d Art completed tasks into the database...\n", len(artCompletedTasks))
	// if len(artCompletedTasks) > 0 {
	// 	collectionmodels.InsertCompletedTaskToDataBase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), artCompletedTasks)
	// }
	// conceptCompletedTasks := FetchAsanaTasksByTeam("Concept", os.Getenv("CLICKUP_SPACE_ID_CONCEPT"))
	// fmt.Printf("Inserting %d Concept completed tasks into the database...\n", len(conceptCompletedTasks))
	// if len(conceptCompletedTasks) > 0 {
	// 	collectionmodels.InsertCompletedTaskToDataBase(db.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), conceptCompletedTasks)
	// }
	// fmt.Println("Weekly ClickUp task synchronization completed.")
}
