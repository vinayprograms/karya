package jira

import "time"

type Issue struct {
	Key    string
	Fields IssueFields
}

type IssueFields struct {
	Summary     string
	Description string
	Status      Status
	Assignee    *User
	DueDate     string // YYYY-MM-DD or empty
	Labels      []string
	IssueType   IssueType
	Parent      *ParentRef
	Subtasks    []Issue
	Comment     CommentPage
}

type Status struct {
	Name           string
	StatusCategory StatusCategory
}

type StatusCategory struct {
	Key string // "new", "indeterminate", "done"
}

type User struct {
	AccountID   string
	DisplayName string
}

type IssueType struct {
	Name    string
	Subtask bool
}

type ParentRef struct {
	Key string
}

type Comment struct {
	Author  User
	Body    string
	Created time.Time
}

type CommentPage struct {
	Comments []Comment
	Total    int
}

// SearchResult represents the paginated response from JIRA search.
type SearchResult struct {
	Issues     []Issue
	StartAt    int
	MaxResults int
	Total      int
}
