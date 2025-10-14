package asana

// Structs for Asana API response
type AsanaResponse struct {
	Data     []Task    `json:"data"`
	NextPage *NextPage `json:"next_page"`
}

type Task struct {
	Gid          string        `json:"gid"`
	Name         string        `json:"name"`
	Completed    bool          `json:"completed"`
	DueOn        string        `json:"due_on"`
	Assignee     *Assignee     `json:"assignee"`
	CustomFields []CustomField `json:"custom_fields"`
}

type Assignee struct {
	Gid   string `json:"gid"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CustomField struct {
	Gid          string `json:"gid"`
	Name         string `json:"name"`
	DisplayValue string `json:"display_value"`
}

type NextPage struct {
	Offset string `json:"offset"`
	Path   string `json:"path"`
	Uri    string `json:"uri"`
}
