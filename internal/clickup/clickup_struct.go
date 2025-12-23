package clickup

type ClickUpAssignee struct {
	Email    string `json:"email"`
	UserName string `json:"username"`
}

type ClickUpOptionItem struct {
	ID         string `json:"id"`
	Name       string `json:"label"`
	OrderIndex int    `json:"orderindex"`
}

type ClickUpOption struct {
	Options []ClickUpOptionItem `json:"options"`
}

type ClickUpToolCustomField struct {
	TypeConfig ClickUpOption `json:"type_config"`
	Value      []string      `json:"value"`
}

type ClickUpLevelCustomField struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type ClickUpCustomField struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Value      any    `json:"value"`
	TypeConfig any    `json:"type_config"`
}

type ClickUpProjectCustomField struct {
	Name       string        `json:"name"`
	TypeConfig ClickUpOption `json:"type_config"`
	Value      int           `json:"value"`
}

type ClickUpTask struct {
	Id           string               `json:"id"`
	Name         string               `json:"name"`
	DateDone     string               `json:"date_done"`
	Assignees    []ClickUpAssignee    `json:"assignees"`
	CustomFields []ClickUpCustomField `json:"custom_fields"`
}

type ClickUpResponse struct {
	Tasks    []ClickUpTask `json:"tasks"`
	LastPage bool          `json:"last_page"`
}

type ClickUpWorkSpaceListResponse struct {
	Lists []ClickUpTaskListResponse `json:"lists"`
}

type ClickUpTaskListResponse struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}
