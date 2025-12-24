package asana

// Response wrapper for fetching lists from a space
type ClickUpListsResponse struct {
	Lists []ClickUpTaskListResponse `json:"lists"`
}

type ClickUpTaskListResponse struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Folder struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Hidden bool   `json:"hidden"`
	Access bool   `json:"access"`
}

type Space struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Access bool   `json:"access"`
}

type Priority struct {
	Priority string `json:"priority"`
	Color    string `json:"color"`
}

// Structs for ClickUp API response
type ClickUpResponse struct {
	Tasks    []Task `json:"tasks"`
	LastPage bool   `json:"last_page"`
}

type Task struct {
	Id           string        `json:"id"`
	Name         string        `json:"name"`
	Status       *Status       `json:"status"`
	DueDate      string        `json:"due_date"`
	Assignees    []Assignee    `json:"assignees"`
	CustomFields []CustomField `json:"custom_fields"`
}

type Status struct {
	Status string `json:"status"`
	Type   string `json:"type"`
}

type Assignee struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CustomField struct {
	Id         string      `json:"id"`
	Name       string      `json:"name"`
	Value      interface{} `json:"value"`
	TypeConfig interface{} `json:"type_config"`
}
