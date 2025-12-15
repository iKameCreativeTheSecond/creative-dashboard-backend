package asana

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
