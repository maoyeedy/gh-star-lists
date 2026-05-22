package domain

// RepoRow is a pre-computed view-model for rendering a repository row.
// Pre-split Owner/Name and pre-formatted display strings avoid repetitive
// computation during rendering.
type RepoRow struct {
	Owner string `json:"-"`
	Name  string `json:"-"`

	Stars     string `json:"-"` // strconv.Itoa(StargazerCount)
	PushedAge string `json:"-"` // humanize.ShortAge(PushedAt, now)
	Fork      string `json:"-"` // "yes" or "no"

	// Raw fields forwarded for JSON, TSV, FZF, Plain output.
	NameWithOwner  string `json:"nameWithOwner"`
	Description    string `json:"description"`
	IsFork         bool   `json:"isFork"`
	IsArchived     bool   `json:"-"`
	StargazerCount int    `json:"stargazerCount"`
	PushedAt       string `json:"pushedAt"`
	URL            string `json:"url"`
	Language       string `json:"language"`
	StarredAt      string `json:"starredAt,omitempty"`
}

// ListRow is a pre-computed view-model for rendering a Star List row.
type ListRow struct {
	RepoCountStr string `json:"-"` // fmt.Sprintf("%d", RepoCount)
	LastAddedAge string `json:"-"` // humanize.ShortAge(LastAddedAt, now)

	// Raw fields forwarded for JSON, TSV, FZF, Plain output.
	Name        string `json:"name"`
	Description string `json:"description"`
	LastAddedAt string `json:"lastAddedAt"`
	IsPrivate   bool   `json:"isPrivate"`
	ID          string `json:"id"`
	RepoCount   int    `json:"repoCount"`
	URL         string `json:"url"`
}
