package githubapi

type listStarListsResponse struct {
	Viewer struct {
		Lists struct {
			Nodes    []starListNode `json:"nodes"`
			PageInfo pageInfo       `json:"pageInfo"`
		} `json:"lists"`
	} `json:"viewer"`
}

type starListItemsConnection struct {
	TotalCount int `json:"totalCount"`
}

type starListNode struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Slug        string                   `json:"slug"`
	Description *string                  `json:"description"`
	LastAddedAt *string                  `json:"lastAddedAt"`
	IsPrivate   bool                     `json:"isPrivate"`
	Items       *starListItemsConnection `json:"items"`
	User        userNode                 `json:"user"`
}

type userNode struct {
	Login string `json:"login"`
}

type pageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}

type listRepositoriesResponse struct {
	Node *userListNode `json:"node"`
}

type userListNode struct {
	Typename string                     `json:"__typename"`
	Items    *repositoryItemsConnection `json:"items"`
}

type repositoryItemsConnection struct {
	Nodes    []*repositoryItemNode `json:"nodes"`
	PageInfo pageInfo              `json:"pageInfo"`
}

type languageNode struct {
	Name string `json:"name"`
}

func (l *languageNode) OrEmpty() string {
	if l == nil {
		return ""
	}
	return l.Name
}

type licenseNode struct {
	Key string `json:"key"`
}

func (l *licenseNode) OrEmpty() string {
	if l == nil {
		return ""
	}
	return l.Key
}

type repositoryTopicConnection struct {
	Nodes []repositoryTopicNode `json:"nodes"`
}

func (c repositoryTopicConnection) Names() []string {
	if len(c.Nodes) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.Nodes))
	for _, node := range c.Nodes {
		if node.Topic.Name != "" {
			names = append(names, node.Topic.Name)
		}
	}
	return names
}

type repositoryTopicNode struct {
	Topic topicNode `json:"topic"`
}

type topicNode struct {
	Name string `json:"name"`
}

type repositoryItemNode struct {
	Typename         string                    `json:"__typename"`
	ID               string                    `json:"id"`
	NameWithOwner    string                    `json:"nameWithOwner"`
	Description      *string                   `json:"description"`
	URL              string                    `json:"url"`
	IsFork           bool                      `json:"isFork"`
	IsArchived       bool                      `json:"isArchived"`
	StargazerCount   int                       `json:"stargazerCount"`
	PushedAt         *string                   `json:"pushedAt"`
	LicenseInfo      *licenseNode              `json:"licenseInfo"`
	RepositoryTopics repositoryTopicConnection `json:"repositoryTopics"`
	PrimaryLanguage  *languageNode             `json:"primaryLanguage"`
}

type listStarredRepositoriesResponse struct {
	Viewer struct {
		StarredRepositories struct {
			Edges    []starredRepositoryEdge `json:"edges"`
			PageInfo pageInfo                `json:"pageInfo"`
		} `json:"starredRepositories"`
	} `json:"viewer"`
}

type starredRepositoryEdge struct {
	StarredAt string              `json:"starredAt"`
	Node      starredRepoItemNode `json:"node"`
}

type starredRepoItemNode struct {
	ID               string                    `json:"id"`
	NameWithOwner    string                    `json:"nameWithOwner"`
	Description      *string                   `json:"description"`
	URL              string                    `json:"url"`
	IsFork           bool                      `json:"isFork"`
	IsArchived       bool                      `json:"isArchived"`
	StargazerCount   int                       `json:"stargazerCount"`
	PushedAt         *string                   `json:"pushedAt"`
	LicenseInfo      *licenseNode              `json:"licenseInfo"`
	RepositoryTopics repositoryTopicConnection `json:"repositoryTopics"`
	PrimaryLanguage  *languageNode             `json:"primaryLanguage"`
}

type getRepositoryResponse struct {
	Repository *repositoryNode `json:"repository"`
}

type repositoryNode struct {
	ID               string                    `json:"id"`
	NameWithOwner    string                    `json:"nameWithOwner"`
	Description      *string                   `json:"description"`
	URL              string                    `json:"url"`
	IsFork           bool                      `json:"isFork"`
	IsArchived       bool                      `json:"isArchived"`
	StargazerCount   int                       `json:"stargazerCount"`
	PushedAt         *string                   `json:"pushedAt"`
	LicenseInfo      *licenseNode              `json:"licenseInfo"`
	RepositoryTopics repositoryTopicConnection `json:"repositoryTopics"`
	PrimaryLanguage  *languageNode             `json:"primaryLanguage"`
}

type getRepositoryIDResponse struct {
	Repository *repositoryIDNode `json:"repository"`
}

type repositoryIDNode struct {
	ID            string `json:"id"`
	NameWithOwner string `json:"nameWithOwner"`
}

type starListMutationResponse struct {
	List starListNode `json:"list"`
}

type createStarListResponse struct {
	CreateUserList starListMutationResponse `json:"createUserList"`
}

type updateStarListResponse struct {
	UpdateUserList starListMutationResponse `json:"updateUserList"`
}
