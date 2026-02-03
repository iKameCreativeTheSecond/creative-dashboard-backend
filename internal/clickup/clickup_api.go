package clickup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"time"

	database "performance-dashboard-backend/internal/database"
	collectionmodels "performance-dashboard-backend/internal/database/collection_models"
	"performance-dashboard-backend/internal/database/constants"
	util "performance-dashboard-backend/internal/utils"

	"github.com/robfig/cron/v3"
)

func FetchTasksFromSpace(token string, spaceID string, isCompleted bool, tag string, includeSubtask bool, fromTimeMilies int64, toTimeMilies int64) ([]ClickUpTask, error) {

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

	isConceptTeam := strings.EqualFold(tag, TAG_CONCEPT_DONE)

	// Fetch tasks from each list
	var allTasks []ClickUpTask
	for _, list := range listsResp.Lists {
		var tasks []ClickUpTask
		var err error
		if isConceptTeam {
			tasks, err = FetchTaskListConcept(token, list.Id, fromTimeMilies, toTimeMilies)
		} else {
			tasks, err = FetchTaskList(token, list.Id, isCompleted, tag, includeSubtask, fromTimeMilies, toTimeMilies)
		}
		if err != nil {
			fmt.Printf("Error fetching tasks from list %s: %v\n", list.Id, err)
			continue
		}
		allTasks = append(allTasks, tasks...)
	}

	return allTasks, nil
}

func FetchTaskListConcept(token string, listID string, fromTimeMilies int64, toTimeMilies int64) ([]ClickUpTask, error) {
	client := &http.Client{}
	page := 0
	var allTasks []ClickUpTask

	for {
		params := []string{"include_closed=true", "archived=false", "tags[]=ccd", fmt.Sprintf("page=%d", page)}
		paramCusfomField := fmt.Sprintf("custom_fields=[{\"field_id\":\"%s\",\"operator\":\">\",\"value\":\"%d\"}]", os.Getenv("CLICKUP_FIELD_ID_CONCEPT_DONE_DATE"), fromTimeMilies)
		params = append(params, paramCusfomField)
		requestURL := fmt.Sprintf("https://api.clickup.com/api/v2/list/%s/task?%s", listID, strings.Join(params, "&"))

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

func FetchTaskList(token string, listID string, isCompleted bool, tag string, includeSubtask bool, fromTimeMilies int64, toTimeMilies int64) ([]ClickUpTask, error) {
	client := &http.Client{}
	page := 0
	var allTasks []ClickUpTask

	for {
		params := []string{"include_closed=true", "archived=false", fmt.Sprintf("page=%d", page)}
		if isCompleted {
			params = append(params, "statuses[]=COMPLETED")
		}
		if includeSubtask {
			params = append(params, "subtasks=true")
		}
		if fromTimeMilies > 0 {
			params = append(params, fmt.Sprintf("date_done_gt=%d", fromTimeMilies))
		}
		if toTimeMilies > 0 {
			params = append(params, fmt.Sprintf("date_done_lt=%d", toTimeMilies))
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
	go ScheduleWeeklyTaskSync()
	// SyncronizeWeeklyClickUpTasksMondayNight()
}

func ScheduleWeeklyTaskSync() {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		fmt.Println("Cannot load Asia/Ho_Chi_Minh, fallback UTC:", err)
		loc = time.UTC
	}
	c := cron.New(cron.WithLocation(loc))
	_, err = c.AddFunc("0 1 * * 2", SyncronizeWeeklyClickUpTasksMondayNight)
	if err != nil {
		fmt.Println("Cron add error:", err)
		return
	}
	_, err = c.AddFunc("0 0 * * 3", SyncronizeWeeklyClickUpTasksTuesdayNight)
	if err != nil {
		fmt.Println("Cron add error:", err)
		return
	}
	c.Start()
}

func SyncronizeWeeklyClickUpTasksTuesdayNight() {
	var wg sync.WaitGroup

	wg.Add(2)
	fmt.Println("Start sync ClickUp tasks at", time.Now())

	go func() {
		defer wg.Done()
		SyncTaskForPlayable()
	}()

	go func() {
		defer wg.Done()
		SyncTaskForVideo()
	}()

	// Đợi tất cả sync tasks hoàn thành trước khi save report
	wg.Wait()

	fmt.Println("Completed sync ClickUp tasks at", time.Now())

	database.SaveProjectReport([]string{constants.Concept, constants.Art})

	fmt.Println("Completed saving project report at", time.Now())
}

func SyncronizeWeeklyClickUpTasksMondayNight() {
	var wg sync.WaitGroup

	wg.Add(2)
	fmt.Println("Start sync ClickUp tasks at", time.Now())
	go func() {
		defer wg.Done()
		SyncTaskForConcept()
	}()

	go func() {
		defer wg.Done()
		SyncTaskForArt()
	}()

	// Đợi tất cả sync tasks hoàn thành trước khi save report
	wg.Wait()

	fmt.Println("Completed sync ClickUp tasks at", time.Now())

	database.SaveProjectReport([]string{constants.Playable, constants.Video})

	fmt.Println("Completed saving project report at", time.Now())
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

	locationVN, locErr := time.LoadLocation("Asia/Ho_Chi_Minh")
	if locErr != nil {
		locationVN = time.FixedZone("ICT", 7*60*60)
	}
	nowVN := time.Now().In(locationVN)
	// Window start: most recent Tuesday 00:00 (local VN time).
	// If that Tuesday is too recent (< 5 days ago), use the previous Tuesday instead.
	weekday := nowVN.Weekday()
	daysSinceTuesday := (int(weekday) - int(time.Tuesday) + 7) % 7
	thisWeekTuesdayStart := time.Date(nowVN.Year(), nowVN.Month(), nowVN.Day(), 0, 0, 0, 0, locationVN).AddDate(0, 0, -daysSinceTuesday)
	windowStartInclusive := thisWeekTuesdayStart
	if nowVN.Sub(thisWeekTuesdayStart) < 5*24*time.Hour {
		windowStartInclusive = thisWeekTuesdayStart.AddDate(0, 0, -7)
	}
	// ClickUp uses date_done_gt (strictly greater). Subtract 1ms so tasks at exactly Tuesday 00:00 are included.
	var windowStartInclusiveMillis = (windowStartInclusive.UnixNano() / int64(time.Millisecond))
	var thisTueDayatMidnightMillis = (thisWeekTuesdayStart.UnixNano() / int64(time.Millisecond))

	// Shift the window from Tuesday 00:00 to Wednesday 00:00 by adding +24h.
	shiftMillis := int64((24 * time.Hour) / time.Millisecond)
	windowStartInclusiveMillis += shiftMillis
	thisTueDayatMidnightMillis += shiftMillis

	var tasks = []*collectionmodels.CompletedTask{}

	var tasks1 = GetTaskForTeam(constants.Playable, os.Getenv("CLICKUP_SPACE_ID_PLA"), "", windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if tasks1 != nil {
		tasks = append(tasks, tasks1...)
	}

	var task2 = GetTaskForTeam(constants.Playable, os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), TAG_PLA, windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if task2 != nil {
		tasks = append(tasks, task2...)
	}

	if len(tasks) > 0 {
		collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), tasks)
	}
}

func SyncTaskForArt() {

	var tasks = []*collectionmodels.CompletedTask{}

	locationVN, locErr := time.LoadLocation("Asia/Ho_Chi_Minh")
	if locErr != nil {
		locationVN = time.FixedZone("ICT", 7*60*60)
	}
	nowVN := time.Now().In(locationVN)
	// Window start: most recent Tuesday 00:00 (local VN time).
	// If that Tuesday is too recent (< 5 days ago), use the previous Tuesday instead.
	weekday := nowVN.Weekday()
	daysSinceTuesday := (int(weekday) - int(time.Tuesday) + 7) % 7
	thisWeekTuesdayStart := time.Date(nowVN.Year(), nowVN.Month(), nowVN.Day(), 0, 0, 0, 0, locationVN).AddDate(0, 0, -daysSinceTuesday)
	windowStartInclusive := thisWeekTuesdayStart
	if nowVN.Sub(thisWeekTuesdayStart) < 5*24*time.Hour {
		windowStartInclusive = thisWeekTuesdayStart.AddDate(0, 0, -7)
	}
	// ClickUp uses date_done_gt (strictly greater). Subtract 1ms so tasks at exactly Tuesday 00:00 are included.
	var windowStartInclusiveMillis = (windowStartInclusive.UnixNano() / int64(time.Millisecond))
	var thisTueDayatMidnightMillis = (thisWeekTuesdayStart.UnixNano() / int64(time.Millisecond))

	var task1 = GetTaskForTeam(constants.Art, os.Getenv("CLICKUP_SPACE_ID_ART"), "", windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if task1 != nil {
		tasks = append(tasks, task1...)
	}

	var task2 = GetTaskForTeam(constants.Art, os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), TAG_CPP, windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if task2 != nil {
		tasks = append(tasks, task2...)
	}

	var task3 = GetTaskForTeam(constants.Art, os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), TAG_ICON, windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if task3 != nil {
		tasks = append(tasks, task3...)
	}

	var task4 = GetTaskForTeam(constants.Art, os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), TAG_BANNER, windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if task4 != nil {
		tasks = append(tasks, task4...)
	}

	var task5 = GetTaskForTeam(constants.Art, os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), TAG_ASSET, windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if task5 != nil {
		tasks = append(tasks, task5...)
	}

	var task6 = GetTaskForTeam(constants.Art, os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), TAG_ART, windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if task6 != nil {
		task6 = dedupeCompletedTasksByTaskName(task6)
		tasks = append(tasks, task6...)
	}

	if len(tasks) > 0 {
		collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), tasks)
	}

}

func SyncTaskForVideo() {

	locationVN, locErr := time.LoadLocation("Asia/Ho_Chi_Minh")
	if locErr != nil {
		locationVN = time.FixedZone("ICT", 7*60*60)
	}
	nowVN := time.Now().In(locationVN)
	// Window start: most recent Tuesday 00:00 (local VN time).
	// If that Tuesday is too recent (< 5 days ago), use the previous Tuesday instead.
	weekday := nowVN.Weekday()
	daysSinceTuesday := (int(weekday) - int(time.Tuesday) + 7) % 7
	thisWeekTuesdayStart := time.Date(nowVN.Year(), nowVN.Month(), nowVN.Day(), 0, 0, 0, 0, locationVN).AddDate(0, 0, -daysSinceTuesday)
	windowStartInclusive := thisWeekTuesdayStart
	if nowVN.Sub(thisWeekTuesdayStart) < 5*24*time.Hour {
		windowStartInclusive = thisWeekTuesdayStart.AddDate(0, 0, -7)
	}
	// ClickUp uses date_done_gt (strictly greater). Subtract 1ms so tasks at exactly Tuesday 00:00 are included.
	var windowStartInclusiveMillis = (windowStartInclusive.UnixNano() / int64(time.Millisecond))
	var thisTueDayatMidnightMillis = (thisWeekTuesdayStart.UnixNano() / int64(time.Millisecond))

	// Shift the window from Tuesday 00:00 to Wednesday 00:00 by adding +24h.
	shiftMillis := int64((24 * time.Hour) / time.Millisecond)
	windowStartInclusiveMillis += shiftMillis
	thisTueDayatMidnightMillis += shiftMillis

	var tasks = []*collectionmodels.CompletedTask{}

	var task1 = GetTaskForTeam(constants.Video, os.Getenv("CLICKUP_SPACE_ID_VIDEO"), "", windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if task1 != nil {
		tasks = append(tasks, task1...)
	}

	var task2 = GetTaskForTeam(constants.Video, os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), TAG_VID, windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if task2 != nil {
		tasks = append(tasks, task2...)
	}

	if len(tasks) > 0 {
		collectionmodels.InsertCompletedTaskToDataBase(database.GetMongoClient(), os.Getenv("MONGODB_NAME"), os.Getenv("MONGODB_COLLECTION_COMPLETED_TASK"), tasks)
	}
}

func GetTaskForTeam(team string, spaceID string, tag string, fromTimeMilies int64, toTimeMilies int64) []*collectionmodels.CompletedTask {

	var includeSubtask bool = false
	if team == constants.Art || team == constants.Video {
		includeSubtask = true
	}

	// fmt.Println("Time Window for team", team, "from", windowStartInclusive, "to", nowVN)
	var res, err = FetchTasksFromSpace(os.Getenv("CLICKUP_TOKEN"), spaceID, true, tag, includeSubtask, fromTimeMilies, toTimeMilies)
	if err != nil {
		fmt.Println("Error fetching ClickUp task list:", err)
		return nil
	}

	var completedTasks []*collectionmodels.CompletedTask

	for _, task := range res {

		var customFieldMap = util.IndexBy(task.CustomFields, func(cf *ClickUpCustomField) string {
			return cf.Name
		})

		if task.DateDone == "" {
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
		if team == constants.Art {
			if tag == "" {
				taskType = "art_asset"
			} else {
				taskType = "art_" + strings.ToLower(tag)
			}
		}

		if taskType == "pla" {
			taskType = "playable"
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

	locationVN, locErr := time.LoadLocation("Asia/Ho_Chi_Minh")
	if locErr != nil {
		locationVN = time.FixedZone("ICT", 7*60*60)
	}
	nowVN := time.Now().In(locationVN)
	// Window start: most recent Tuesday 00:00 (local VN time).
	// If that Tuesday is too recent (< 5 days ago), use the previous Tuesday instead.
	weekday := nowVN.Weekday()
	daysSinceTuesday := (int(weekday) - int(time.Tuesday) + 7) % 7
	thisWeekTuesdayStart := time.Date(nowVN.Year(), nowVN.Month(), nowVN.Day(), 0, 0, 0, 0, locationVN).AddDate(0, 0, -daysSinceTuesday)
	windowStartInclusive := thisWeekTuesdayStart
	if nowVN.Sub(thisWeekTuesdayStart) < 5*24*time.Hour {
		windowStartInclusive = thisWeekTuesdayStart.AddDate(0, 0, -7)
	}
	// ClickUp uses date_done_gt (strictly greater). Subtract 1ms so tasks at exactly Tuesday 00:00 are included.
	var windowStartInclusiveMillis = (windowStartInclusive.UnixNano() / int64(time.Millisecond))
	var thisTueDayatMidnightMillis = (thisWeekTuesdayStart.UnixNano() / int64(time.Millisecond))
	fmt.Println("Time Window for Concept team from", windowStartInclusive, "to", thisWeekTuesdayStart)

	var res, err = FetchTasksFromSpace(os.Getenv("CLICKUP_TOKEN"), os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), true, TAG_CONCEPT_DONE, false, windowStartInclusiveMillis, thisTueDayatMidnightMillis)
	if err != nil {
		fmt.Println("Error fetching ClickUp task list:", err)
		return nil
	}

	var completedTasks []*collectionmodels.CompletedTask

	for _, task := range res {

		var customFieldMap = util.IndexBy(task.CustomFields, func(cf *ClickUpCustomField) string {
			return cf.Name
		})

		var toolIndexes []int
		if toolCustomField, ok := customFieldMap["Tool/CTST Concept"]; ok && toolCustomField != nil {
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

		var conceptDoneDate, okDate = customFieldMap["Ngày tick Done Concept"]
		if !okDate || conceptDoneDate.Value == nil {
			fmt.Println("Concept Done Date custom field missing for task:", task.Name)
			continue
		}

		conceptDoneDateMillis, ok1 := anyToInt64(conceptDoneDate.Value)
		if !ok1 {
			fmt.Println("Error converting Concept Done Date custom field to int64 for task:", task.Name)
			continue
		}
		if conceptDoneDateMillis == 0 || conceptDoneDateMillis > thisTueDayatMidnightMillis {
			fmt.Println("Concept Done Date out of range for task:", task.Name)
			continue
		}

		var difficultCustomField, okLevel = customFieldMap["Concept Difficult"]

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
			assigneeEmail = task.Assignees[0].Email
		}

		var level, ok = anyToInt(difficultCustomField.Value)
		if !ok {
			fmt.Println("Error converting level value to int for task:", task.Name)
			continue
		}

		var completedTask = &collectionmodels.CompletedTask{
			TaskID:     task.Id,
			TaskName:   task.Name,
			AssigneeID: assigneeEmail,
			Tool:       toolIndexes,
			Level:      level,
			Project:    projectName,
			Team:       constants.Concept,
			TaskType:   TAG_CONCEPT,
			DoneDate:   GetMondayAtNineAM(),
		}
		completedTasks = append(completedTasks, completedTask)
	}
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

func anyToInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case float64:
		// encoding/json decodes numbers into float64 when the destination is `any`.
		return int64(t), true
	case json.Number:
		i64, err := t.Int64()
		if err != nil {
			return 0, false
		}
		return i64, true
	case string:
		if t == "" {
			return 0, false
		}
		i64, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, false
		}
		return i64, true
	default:
		return 0, false
	}
}

func dedupeCompletedTasksByTaskName(tasks []*collectionmodels.CompletedTask) []*collectionmodels.CompletedTask {
	if len(tasks) == 0 {
		return tasks
	}

	seen := make(map[string]struct{}, len(tasks))
	out := make([]*collectionmodels.CompletedTask, 0, len(tasks))

	for _, task := range tasks {
		if task == nil {
			continue
		}
		nameKey := strings.ToLower(strings.TrimSpace(task.TaskName))
		if nameKey == "" {
			out = append(out, task)
			continue
		}
		if _, ok := seen[nameKey]; ok {
			continue
		}
		seen[nameKey] = struct{}{}
		out = append(out, task)
	}

	return out
}
