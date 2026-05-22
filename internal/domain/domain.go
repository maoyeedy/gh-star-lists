// Package domain defines the core domain types for gh-star-lists.
// This package must have zero internal imports -- it is the leaf of the
// dependency graph.
package domain

// ListOptions control pagination and feature flags for list operations.
type ListOptions struct {
	Limit      int
	WithTopics bool
}

// StarList represents a GitHub Star List.
type StarList struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	LastAddedAt     string `json:"lastAddedAt"`
	IsPrivate       bool   `json:"isPrivate"`
	ID              string `json:"id"`
	RepoCount       int    `json:"repoCount"`
	URL             string `json:"url"`
	NormName        string `json:"-"`
	NormDescription string `json:"-"`
}

// Repository represents a GitHub repository.
type Repository struct {
	ID                string   `json:"-"`
	NameWithOwner     string   `json:"nameWithOwner"`
	Description       string   `json:"description"`
	IsFork            bool     `json:"isFork"`
	StargazerCount    int      `json:"stargazerCount"`
	PushedAt          string   `json:"pushedAt"`
	URL               string   `json:"url"`
	Language          string   `json:"language"`
	StarredAt         string   `json:"starredAt,omitempty"`
	IsArchived        bool     `json:"-"`
	License           string   `json:"-"`
	Topics            []string `json:"-"`
	NormNameWithOwner string   `json:"-"`
	NormDescription   string   `json:"-"`
	NormLanguage      string   `json:"-"`
}

// StarListInput holds the fields for creating a new Star List.
type StarListInput struct {
	Name        string
	Description string
	Private     bool
}

// UpdateStarListInput holds the fields for updating an existing Star List.
type UpdateStarListInput struct {
	ID          string
	Name        string
	Description string
	Private     *bool
}
