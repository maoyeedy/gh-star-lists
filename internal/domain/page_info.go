package domain

// PageInfo holds cursor-based pagination information.
type PageInfo struct {
	HasNextPage bool
	EndCursor   string
}
