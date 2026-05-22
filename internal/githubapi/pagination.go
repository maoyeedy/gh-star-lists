package githubapi

import "context"

// Pager paginates over a GraphQL connection. The fetch callback executes one
// page request and returns its items along with the pageInfo struct. Pager
// handles ctx.Err(), pageFirst calculation, limitReached checks, and the
// loop.
func Pager[T any](
	ctx context.Context,
	pageSize int,
	limit int,
	fetch func(endCursor any, first int) ([]T, pageInfo, error),
) ([]T, error) {
	result := make([]T, 0, pageSize)
	var endCursor any

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		nodes, pi, err := fetch(endCursor, pageFirst(pageSize, limit, len(result)))
		if err != nil {
			return nil, err
		}

		result = append(result, nodes...)
		if limit > 0 && len(result) > limit {
			result = result[:limit]
		}

		if limitReached(limit, len(result)) || !pi.HasNextPage {
			return result, nil
		}
		endCursor = stringValue(pi.EndCursor)
	}
}

func pageFirst(pageSize, limit, current int) int {
	if limit <= 0 {
		return pageSize
	}
	remaining := limit - current
	if remaining <= 0 {
		return 1
	}
	if remaining < pageSize {
		return remaining
	}
	return pageSize
}

func limitReached(limit, count int) bool {
	return limit > 0 && count >= limit
}
