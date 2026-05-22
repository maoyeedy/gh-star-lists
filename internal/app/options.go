package app

type Filter struct {
	Key   string
	Value string
}

type SortTerm struct {
	Key  string
	Desc bool
}

type ListListsOptions struct {
	Filters   []Filter
	SortKeys  []string
	SortTerms []SortTerm
	SortDesc  bool
	Limit     int
}

type ListReposOptions struct {
	All       bool
	Unlisted  bool
	Filters   []Filter
	SortKeys  []string
	SortTerms []SortTerm
	SortDesc  bool
	Limit     int
	Search    string
	Topics    bool
}
