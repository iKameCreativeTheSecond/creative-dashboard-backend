package clickup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func FetchTasksFromSpace(token string, spaceID string, isCompleted bool) ([]ClickUpTask, error) {

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
	body, _ := io.ReadAll(resp.Body)

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
		tasks, err := FetchTaskList(token, list.Id, isCompleted)
		if err != nil {
			fmt.Printf("Error fetching tasks from list %s: %v\n", list.Id, err)
			continue
		}
		allTasks = append(allTasks, tasks...)
	}

	return allTasks, nil
}

func FetchTaskList(token string, listID string, isCompleted bool) ([]ClickUpTask, error) {
	// Build ClickUp API URL
	var url string
	urlCompltetedFilter := fmt.Sprintf("https://api.clickup.com/api/v2/list/%s/task?statuses[]=COMPLETED&include_closed=true&archived=false", listID)
	urlOpenFilter := fmt.Sprintf("https://api.clickup.com/api/v2/list/%s/task?include_closed=false&archived=false", listID)
	if isCompleted {
		url = urlCompltetedFilter
	} else {
		url = urlOpenFilter
	}
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
	var listsResp ClickUpResponse
	if err := json.Unmarshal(body, &listsResp); err != nil {
		fmt.Printf("Error unmarshalling ClickUp response: %v\nResponse body: %s\n", err, string(body))
		return nil, err
	}

	return listsResp.Tasks, nil
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
	// Placeholder for any initialization logic if needed in the future

}

func SyncTaskForConcept() {
	var res, err = FetchTasksFromSpace(os.Getenv("CLICKUP_TOKEN"), os.Getenv("CLICKUP_SPACE_ID_CONCEPT"), true)
	if err != nil {
		fmt.Println("Error fetching ClickUp task list:", err)
		return
	}

	fmt.Printf("Fetched %d completed tasks from ClickUp.\n", len(res))

	for _, task := range res {
		fmt.Printf("Task ID: %s, Name: %s, Date Done: %s\n", task.Id, task.Name, task.DateDone)
	}
}
