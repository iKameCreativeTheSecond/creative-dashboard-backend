package clickup

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	database "performance-dashboard-backend/internal/database"
	collectionmodels "performance-dashboard-backend/internal/database/collection_models"
	util "performance-dashboard-backend/internal/utils"

	"github.com/robfig/cron/v3"
)

func FetchTasksFromSpace(token string, spaceID string, isCompleted bool, tag string, includeSubtask bool) ([]ClickUpTask, error) {

	url := fmt.Sprintf("https://api.clickup.com/api/v2/space/%s/list", spaceID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error Fetching List Task in Workspace", err)
		return nil, err
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request to ClickUp API:", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read body
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		fmt.Printf("Error reading ClickUp response body (spaceID=%s, status=%d): %v\n", spaceID, resp.StatusCode, readErr)
		return nil, readErr
	}

	// Debug: Print response status and body
	fmt.Printf("ClickUp API Response Status: %d\n", resp.StatusCode)

	// Parse JSON - the response has a "lists" wrapper
	var listsResp ClickUpWorkSpaceListResponse
	if err := json.Unmarshal(body, &listsResp); err != nil {
		fmt.Printf("Error unmarshalling ClickUp workspace list response: %v\nResponse body: %s\n", err, string(body))
		return nil, err
	}

	// Fetch tasks from each list
	var allTasks []ClickUpTask
	for _, list := range listsResp.Lists {
		tasks, err := FetchTaskList(token, list.Id, isCompleted, tag, includeSubtask)
		if err != nil {
			fmt.Printf("Error fetching tasks from list %s: %v\n", list.Id, err)
			continue
		}
		allTasks = append(allTasks, tasks...)
	}

	return allTasks, nil
}

func FetchTaskList(token string, listID string, isCompleted bool, tag string, includeSubtask bool) ([]ClickUpTask, error) {
	client := &http.Client{}
	page := 0
	var allTasks []ClickUpTask

	for {
		params := []string{"include_closed=true", "archived=false"}
		if isCompleted {
			params = append(params, "statuses[]=COMPLETED")
		}
		if includeSubtask {
			params = append(params, "subtasks=true")
		}
		requestURL := fmt.Sprintf("https://api.clickup.com/api/v2/list/%s/task?%s", listID, strings.Join(params, "&"))

		tagTrim := strings.TrimSpace(tag)
		if tagTrim != "" {
			requestURL += fmt.Sprintf("&tags[]=%s", neturl.QueryEscape(tagTrim))
		}

		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			fmt.Printf("Error creating ClickUp request (listID=%s, page=%d): %v\n", listID, page, err)
			return nil, err
		}
		req.Header.Set("Authorization", token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error sending ClickUp request (listID=%s, page=%d): %v\n", listID, page, err)
			return nil, err
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			fmt.Printf("Error reading ClickUp response body (listID=%s, page=%d, status=%d): %v\n", listID, page, resp.StatusCode, readErr)
			return nil, readErr
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("clickup list tasks request failed (listID=%s, page=%d, status=%d): %s", listID, page, resp.StatusCode, string(body))
		}

		// Parse JSON
		var listResp ClickUpResponse
		if err := json.Unmarshal(body, &listResp); err != nil {
			fmt.Printf("Error unmarshalling ClickUp response (listID=%s, page=%d): %v\nResponse body: %s\n", listID, page, err, string(body))
			return nil, err
		}

		allTasks = append(allTasks, listResp.Tasks...)
		if listResp.LastPage || len(listResp.Tasks) == 0 {
			break
		}
		page++
	}

	return allTasks, nil
}

func UnixMillisToTime(ms int64) time.Time {
	sec := ms / 1000
	nsec := (ms % 1000) * int64(time.Millisecond)
	return time.Unix(sec, nsec).UTC()
}

func UnixMillisToTimeStr(msStr string) time.Time {
	var ms int64
	fmt.Sscanf(msStr, "%d", &ms)
	return UnixMillisToTime(ms)
}

func Init() {
	// go ScheduleWeeklyTaskSync()
	// go SyncronizeWeeklyClickUpTasks()
}

func ScheduleWeeklyTaskSync() {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		fmt.Println("Cannot load Asia/Ho_Chi_Minh, fallback UTC:", err)
		loc = time.UTC
	}
	c := cron.New(cron.WithLocation(loc))
	_, err = c.AddFunc("59 23 * * 1", SyncronizeWeeklyClickUpTasks)
	if err != nil {
		fmt.Println("Cron add error:", err)
		return
	}
	c.Start()
}

func SyncronizeWeeklyClickUpTasks() {
	// go SyncTaskForConcept()
	// go SyncTaskForPlayable()
	// go SyncTaskForArt()
	// go SyncTaskForVideo()

	// go database.SaveProjectReport()
}

func SyncTaskForConcept() {
	var tasks = GetTaskForConcept()
	if tasks != nil {
		if len(tasks) > 0 {
			collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), tasks)
		}
	}
}

func SyncTaskForPlayable() {
	var tasks = GetTaskForTeam("PLA", os.Getenv("CLICKUP_SPACE_ID_PLA"), "")
	if tasks != nil {
		if len(tasks) > 0 {
			collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), tasks)
		}
	}

	var task2 = GetTaskForTeam("PLA", os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), PLA)
	if task2 != nil {
		if len(task2) > 0 {
			collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), task2)
		}
	}
}

func SyncTaskForArt() {
	var tasks = GetTaskForTeam("Art", os.Getenv("CLICKUP_SPACE_ID_ART"), "")
	if tasks != nil {
		if len(tasks) > 0 {
			collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), tasks)
		}
	}

	var task2 = GetTaskForTeam("Art", os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), CPP)
	if task2 != nil {
		if len(task2) > 0 {
			collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), task2)
		}
	}

	var task3 = GetTaskForTeam("Art", os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), ICON)
	if task3 != nil {
		if len(task3) > 0 {
			collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), task3)
		}
	}

	var task4 = GetTaskForTeam("Art", os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), BANNER)
	if task4 != nil {
		if len(task4) > 0 {
			collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), task4)
		}
	}
}

func SyncTaskForVideo() {
	var tasks = GetTaskForTeam("Video", os.Getenv("CLICKUP_SPACE_ID_VIDEO"), "")
	if tasks != nil {
		if len(tasks) > 0 {
			collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), tasks)
		}
	}

	var task2 = GetTaskForTeam("Video", os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), VID)
	if task2 != nil {
		if len(task2) > 0 {
			collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), task2)
		}
	}
}

func GetTaskForTeam(team string, spaceID string, tag string) []*collectionmodels.CompletedTask {

	var res, err = FetchTasksFromSpace(os.Getenv("CLICKUP_TOKEN"), spaceID, true, tag, true)
	if err != nil {
		fmt.Println("Error fetching ClickUp task list:", err)
		return nil
	}

	locationVN, locErr := time.LoadLocation("Asia/Ho_Chi_Minh")
	if locErr != nil {
		locationVN = time.FixedZone("ICT", 7*60*60)
	}
	nowVN := time.Now().In(locationVN)
	// Window: 00:01 Tuesday last week -> 23:59 Monday this week (end-exclusive: Tuesday 00:00 this week)
	weekday := nowVN.Weekday()
	daysSinceTuesday := (int(weekday) - int(time.Tuesday) + 7) % 7
	thisWeekTuesdayStart := time.Date(nowVN.Year(), nowVN.Month(), nowVN.Day(), 0, 0, 0, 0, locationVN).AddDate(0, 0, -daysSinceTuesday)
	// windowEndExclusive := thisWeekTuesdayStart
	windowStartInclusive := thisWeekTuesdayStart.AddDate(0, 0, -7).Add(1 * time.Minute)

	var completedTasks []*collectionmodels.CompletedTask

	for _, task := range res {

		var customFieldMap = util.IndexBy(task.CustomFields, func(cf *ClickUpCustomField) string {
			return cf.Name
		})

		if task.DateDone == "" {
			continue
		}

		taskDoneDate := UnixMillisToTimeStr(task.DateDone).In(locationVN)
		if taskDoneDate.Before(windowStartInclusive) || taskDoneDate.After(nowVN) {
			continue
		}

		var toolIndexes []int
		if toolCustomField, ok := customFieldMap["Tool/CTST "+team]; ok && toolCustomField != nil {
			toolFields, err := util.CoerceStruct[ClickUpToolCustomField](toolCustomField)
			if err == nil {
				var options = toolFields.TypeConfig.Options
				for _, selectedToolID := range toolFields.Value {
					for _, option := range options {
						if option.ID == selectedToolID {
							var inx = GetToolIndex(option.Name)
							if inx != -1 {
								toolIndexes = append(toolIndexes, inx)
							}
							break
						}
					}
				}
			}
		}

		var difficultCustomField, okLevel = customFieldMap[team+" Difficult"]

		if !okLevel || difficultCustomField.Value == nil {
			fmt.Println("Difficult custom field missing for task:", task.Name)
			continue
		}

		var projecCustomField, okProject = customFieldMap["Game Name"]
		if !okProject || projecCustomField.Value == nil {
			fmt.Println("Project custom field missing for task:", task.Name)
			continue
		}

		var projectField, err = util.CoerceStruct[ClickUpProjectCustomField](projecCustomField)
		if err != nil {
			fmt.Println("Error coercing project custom field for task:", task.Name)
			continue
		}

		projectIndex := projectField.Value
		if projectIndex < 0 || projectIndex >= len(projectField.TypeConfig.Options) {
			fmt.Println("Invalid project index for task:", task.Name)
			continue
		}
		projectName := projectField.TypeConfig.Options[projectIndex].Name
		spaceIndex := strings.Index(projectName, " ")
		if spaceIndex != -1 {
			projectName = projectName[spaceIndex+1:]
		}

		var assigneeEmail string = ""
		if len(task.Assignees) > 0 {
			assigneeIndex := 0
			if len(task.Assignees) > 1 {
				assigneeIndex = 1
			}
			assigneeEmail = task.Assignees[assigneeIndex].Email
		}

		var level, ok = anyToInt(difficultCustomField.Value)
		if !ok {
			fmt.Println("Error converting level value to int for task:", task.Name)
			continue
		}

		var taskType string = strings.ToLower(team)
		if team == "Art" {
			taskType = "art_" + strings.ToLower(tag)
		}

		var completedTask = &collectionmodels.CompletedTask{
			TaskID:     task.Id,
			TaskName:   task.Name,
			AssigneeID: assigneeEmail,
			Tool:       toolIndexes,
			Level:      level,
			Project:    projectName,
			Team:       team,
			TaskType:   taskType,
			DoneDate:   GetMondayAtNineAM(),
		}
		completedTasks = append(completedTasks, completedTask)
	}
	return completedTasks
}

func GetTaskForConcept() []*collectionmodels.CompletedTask {
	var res, err = FetchTasksFromSpace(os.Getenv("CLICKUP_TOKEN"), os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), false, CONCEPT_DONE, true)
	if err != nil {
		fmt.Println("Error fetching ClickUp task list:", err)
		return nil
	}

	fmt.Printf("Fetched %d completed tasks from ClickUp.\n", len(res))

	locationVN, locErr := time.LoadLocation("Asia/Ho_Chi_Minh")
	if locErr != nil {
		locationVN = time.FixedZone("ICT", 7*60*60)
	}
	nowVN := time.Now().In(locationVN)
	// Window: 00:01 Tuesday last week -> 23:59 Monday this week (end-exclusive: Tuesday 00:00 this week)
	weekday := nowVN.Weekday()
	daysSinceTuesday := (int(weekday) - int(time.Tuesday) + 7) % 7
	thisWeekTuesdayStart := time.Date(nowVN.Year(), nowVN.Month(), nowVN.Day(), 0, 0, 0, 0, locationVN).AddDate(0, 0, -daysSinceTuesday)
	// windowEndExclusive := thisWeekTuesdayStart
	windowStartInclusive := thisWeekTuesdayStart.AddDate(0, 0, -7).Add(1 * time.Minute)

	var completedTasks []*collectionmodels.CompletedTask

	for _, task := range res {
		debugSkip := func(reason string, err error) {
			if err != nil {
				log.Printf("[ClickUp][Concept] skip task id=%s name=%q reason=%s err=%v", task.Id, task.Name, reason, err)
				return
			}
			log.Printf("[ClickUp][Concept] skip task id=%s name=%q reason=%s", task.Id, task.Name, reason)
		}

		fmt.Printf("Task ID: %s, Name: %s\n", task.Id, task.Name)

		var customFieldMap = util.IndexBy(task.CustomFields, func(cf *ClickUpCustomField) string {
			return cf.Name
		})

		dayTickDoneCustomField, ok := customFieldMap["Ngày tick Done Concept"]
		if !ok || dayTickDoneCustomField.Value == nil {
			debugSkip("missing custom field: Ngày tick Done Concept", nil)
			continue
		}

		taskDoneDate := UnixMillisToTimeStr(dayTickDoneCustomField.Value.(string)).In(locationVN)

		fmt.Printf("Task Done Date: %v , Before %v , After %v\n", taskDoneDate, windowStartInclusive, nowVN)

		if taskDoneDate.Before(windowStartInclusive) || taskDoneDate.After(nowVN) {
			debugSkip("task.DateDone outside time window", nil)
			continue
		}

		var toolIndexes []int
		if toolCustomField, ok := customFieldMap["Tool/CTST Concept"]; ok && toolCustomField != nil {
			toolFields, err := util.CoerceStruct[ClickUpToolCustomField](toolCustomField)
			if err != nil {
				debugSkip("failed to parse Tool/CTST Concept custom field", err)
			} else {
				var options = toolFields.TypeConfig.Options
				for _, selectedToolID := range toolFields.Value {
					for _, option := range options {
						if option.ID == selectedToolID {
							var inx = GetToolIndex(option.Name)
							if inx != -1 {
								toolIndexes = append(toolIndexes, inx)
							}
							break
						}
					}
				}
			}
		}

		var conceptDifficultCustomField, okLevel = customFieldMap["Concept Difficult"]
		if !okLevel || conceptDifficultCustomField.Value == nil {
			debugSkip("missing custom field: Concept Difficult", nil)
			continue
		}

		var projecCustomField, okProject = customFieldMap["Game Name"]
		if !okProject || projecCustomField.Value == nil {
			debugSkip("missing custom field: Game Name", nil)
			continue
		}

		var projectField, err = util.CoerceStruct[ClickUpProjectCustomField](projecCustomField)
		if err != nil {
			debugSkip("failed to parse Game Name custom field", err)
			continue
		}

		projectIndex := projectField.Value
		if projectIndex < 0 || projectIndex >= len(projectField.TypeConfig.Options) {
			debugSkip(fmt.Sprintf("invalid Game Name index: %d", projectIndex), nil)
			continue
		}
		projectName := projectField.TypeConfig.Options[projectIndex].Name
		spaceIndex := strings.Index(projectName, " ")
		if spaceIndex != -1 {
			projectName = projectName[spaceIndex+1:]
		}

		level, ok := anyToInt(conceptDifficultCustomField.Value)
		if !ok {
			debugSkip("failed to convert Concept Difficult value to int", nil)
			continue
		}

		assigneeEmail := task.Assignees[0].Email
		var completedTask = &collectionmodels.CompletedTask{
			TaskID:     task.Id,
			TaskName:   task.Name,
			AssigneeID: assigneeEmail,
			Tool:       toolIndexes,
			Level:      level,
			Project:    projectName,
			Team:       "Concept",
			TaskType:   "Concept",
			DoneDate:   GetMondayAtNineAM(),
		}
		completedTasks = append(completedTasks, completedTask)
	}
	fmt.Printf("Total processed completed tasks for Concept: %d\n", len(completedTasks))
	return completedTasks
}

func GetToolIndex(toolName string) int {
	re := regexp.MustCompile(`^\d+`)
	match := re.FindString(toolName)

	if match != "" {
		num, _ := strconv.Atoi(match)
		return num
	}
	return -1
}

func GetMondayAtNineAM() time.Time {
	locationVN, locErr := time.LoadLocation("Asia/Ho_Chi_Minh")
	if locErr != nil {
		locationVN = time.FixedZone("ICT", 7*60*60)
	}
	nowVN := time.Now().In(locationVN)
	weekday := int(nowVN.Weekday())
	daysSinceMonday := (weekday + 6) % 7
	mondayDate := nowVN.AddDate(0, 0, -daysSinceMonday)
	thisMondayAtNine := time.Date(mondayDate.Year(), mondayDate.Month(), mondayDate.Day(), 9, 0, 0, 0, locationVN)
	return thisMondayAtNine
}

func anyToInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		// encoding/json decodes numbers into float64 when the destination is `any`.
		return int(t), true
	case json.Number:
		i64, err := t.Int64()
		if err != nil {
			return 0, false
		}
		return int(i64), true
	case string:
		if t == "" {
			return 0, false
		}
		i, err := strconv.Atoi(t)
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}
