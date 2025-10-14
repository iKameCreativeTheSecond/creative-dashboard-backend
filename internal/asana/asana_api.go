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

	"go.mongodb.org/mongo-driver/mongo"
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

func FetchAsanaTasksTeamPlayable(team string, projectID string) []*collectionmodels.CompletedTask {
	token := os.Getenv("ASANA_TOKEN") // safer to set as env var
	tasks, err := FetchTasks(token, projectID)
	if err != nil {
		fmt.Println("Error:", err)
		return nil
	}

	var completedTasks []*collectionmodels.CompletedTask
	var thisMondayAtNine time.Time
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	thisMondayAtNine = now.AddDate(0, 0, -weekday+1).Truncate(24 * time.Hour).Add(9 * time.Hour)
	for _, task := range tasks {

		if !task.Completed {
			continue
		}

		// Map Asana task to internal model
		var toolIndexes []int
		var level int
		var projectName string
		for _, field := range task.CustomFields {
			if field.Name == "Tool/CTST PLA" {
				toolIndexes = GetListToolAsIndexes(field.DisplayValue)
			}
			if field.Name == "PLA Difficult" {
				if lvl, err := strconv.Atoi(field.DisplayValue); err == nil {
					level = lvl
				}
			}
			if field.Name == "Game Name" {
				projectName = field.DisplayValue
			}
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

		fmt.Print("ID ", completedTask.TaskID, " | Name: ", completedTask.TaskName, " | Assignee: ", completedTask.AssigneeID, " | Tool: ", completedTask.Tool, " | Level: ", completedTask.Level, " | Project: ", completedTask.Project, " | DoneDate: ", completedTask.DoneDate.Format("2006-01-02"), "\n")

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
