# Architecture Audit & Refactor -- Ship Record

## What shipped

Comprehensive refactor of the gh-star-lists Go codebase to establish proper
layered architecture: domain types extracted to a zero-dependency leaf
package, pagination deduplicated with a generic helper, an application service
layer introduced between CLI and API, string-based auth error detection
replaced with typed domain errors and exit-code mapping, and view-model types
added for renderers with retry lifted from transport-level to Service-level
decorator.

- **Domain package extracted.** `StarList`, `Repository`, `PageInfo`, input
  types, and typed errors now live in `internal/domain` -- a zero-dependency
  leaf. All packages (`command`, `format`, `tui`, `search`, `githubapi`)
  import `domain` for types instead of coupling to `githubapi`.
- **Generic pagination.** Three duplicate `for { ... HasNextPage ... }` loops
  in `graphql_service.go` replaced with a single `Pager[T]` helper, removing
  ~60 lines of duplicated control flow.
- **Application service layer.** New `internal/app` package with
  `StarListService` orchestrates use cases (list, filter, sort, limit).
  `command/run.go` action cases thinned to parse->call->format; filter/sort
  logic moved from `command` to `app`.
- **Typed error taxonomy.** `domain.AuthError`, `domain.NotFoundError`,
  `domain.RateLimitError` with `errors.Is`/`errors.As` support. GraphQL error
  normalization at the `githubapi` boundary. String-based
  `looksLikeAuthError`/`authMarkers` removed from `command/run_output.go` in
  favor of typed checks. Single `mapErrorToExitCode(err) int` function.
- **View-model types.** `RepoRow` (pre-split owner/name, pre-formatted stars,
  age, fork) and `ListRow` (pre-formatted counts, age) in `internal/domain`,
  with constructors in `internal/format/rows.go`. Format and TUI renderers
  consume rows instead of raw domain types.
- **Retry at Service level.** `RetryService` decorator wraps `Service`,
  retries transient failures on read methods, passes through mutations. Wired
  into production stack: `lazyService -> RetryService -> cacheService ->
  diskCacheService -> graphQLService`. Legacy `retryDoer` at HTTP transport
  level removed.

## Files changed

| File | Change |
|------|--------|
| `internal/domain/domain.go` | Added -- core domain types (StarList, Repository, input types) |
| `internal/domain/page_info.go` | Added -- PageInfo struct |
| `internal/domain/errors.go` | Added -- typed sentinel errors and error types |
| `internal/domain/rows.go` | Added -- RepoRow, ListRow view-model types |
| `internal/githubapi/pagination.go` | Added -- generic Pager[T] helper, pageFirst, limitReached |
| `internal/githubapi/errors.go` | Added -- normalizeError from HTTP/GraphQL errors to domain types |
| `internal/githubapi/retry_service.go` | Added -- RetryService decorator implementing Service |
| `internal/app/service.go` | Added -- StarListService orchestration with ListLists, ListRepos, mutation pass-throughs |
| `internal/app/options.go` | Added -- ListListsOptions, ListReposOptions, Filter, SortTerm |
| `internal/app/filter.go` | Added -- filterStarLists, filterRepositories (moved from command) |
| `internal/app/sort.go` | Added -- SortStarLists, SortRepositories (moved from command) |
| `internal/format/rows.go` | Added -- RepoRowFromDomain, ListRowFromDomain constructors |
| `internal/githubapi/client.go` | Modified -- Service interface references domain types; RetryService wired into production stack |
| `internal/githubapi/graphql_service.go` | Modified -- DTO->domain mapping targets domain types; pagination loops replaced with Pager calls; normalizeError applied at DoWithContext boundaries |
| `internal/githubapi/graphql_helpers.go` | Modified -- pageFirst, limitReached removed (folded into pagination.go) |
| `internal/githubapi/cache.go` | Modified -- cache methods use domain types |
| `internal/githubapi/diskcache.go` | Modified -- cache methods use domain types |
| `internal/githubapi/diskcache_store.go` | Modified -- cache methods use domain types |
| `internal/githubapi/membership_index.go` | Modified -- updated to domain types |
| `internal/githubapi/starred_at.go` | Modified -- updated to domain types |
| `internal/githubapi/retry.go` | Modified -- retryDoer removed (replaced by RetryService) |
| `internal/githubapi/client_test.go` | Modified -- updated to domain types |
| `internal/githubapi/cache_test.go` | Modified -- updated to domain types |
| `internal/githubapi/diskcache_test.go` | Modified -- updated to domain types |
| `internal/githubapi/graphql_test.go` | Modified -- updated to domain types |
| `internal/githubapi/starred_at_test.go` | Modified -- updated to domain types |
| `internal/githubapi/retry_test.go` | Modified -- moved retryDoer to test-local |
| `internal/command/run.go` | Modified -- action cases thinned to parse->call app->format |
| `internal/command/run_action.go` | Modified -- fetchReposForAction moved to app; delegates to StarListService |
| `internal/command/run_filter.go` | Modified -- filter functions moved to app |
| `internal/command/run_sort.go` | Modified -- sort functions moved to app |
| `internal/command/run_output.go` | Modified -- string-based auth detection replaced with typed errors + exit-code mapping |
| `internal/command/run_tui.go` | Modified -- updated to domain types |
| `internal/command/run_prompt.go` | Modified -- updated to domain types |
| `internal/command/search.go` | Modified -- updated to domain types |
| `internal/command/run_test.go` | Modified -- updated to domain types |
| `internal/command/run_bench_test.go` | Modified -- updated to app sort functions |
| `internal/command/search_test.go` | Modified -- updated to domain types |
| `internal/format/repositories.go` | Modified -- render functions accept []RepoRow |
| `internal/format/star_lists.go` | Modified -- render functions accept []ListRow |
| `internal/format/repositories_test.go` | Modified -- updated to domain types |
| `internal/format/star_lists_test.go` | Modified -- updated to domain types |
| `internal/search/search.go` | Modified -- updated to domain types |
| `internal/search/scoring.go` | Modified -- updated to domain types |
| `internal/search/search_test.go` | Modified -- updated to domain types |
| `internal/search/search_bench_test.go` | Modified -- updated to domain types |
| `internal/tui/model.go` | Modified -- updated to domain types |
| `internal/tui/messages.go` | Modified -- updated to domain types |
| `internal/tui/cache.go` | Modified -- updated to domain types |
| `internal/tui/sort.go` | Modified -- updated to domain types |
| `internal/tui/render_repo.go` | Modified -- render functions accept RepoRow |
| `internal/tui/render_list.go` | Modified -- render functions use ListRow |
| `internal/tui/render_preview.go` | Modified -- updated to domain types |
| `internal/tui/modal.go` | Modified -- updated to domain types |
| `internal/tui/modal_list.go` | Modified -- updated to domain types |
| `internal/tui/modal_repo.go` | Modified -- updated to domain types |
| `internal/tui/modal_bulk.go` | Modified -- updated to domain types |
| `internal/tui/app.go` | Modified -- updated to domain types |
| `internal/tui/test_helpers_test.go` | Modified -- updated to domain types |

## Design notes

- **Pager callback returns pageInfo by value, not pointer.** The Pager
  helper's fetch closure returns `pageInfo` (value type) rather than
  `*pageInfo`. This avoids heap allocation in the hot path and keeps the
  callback signature consistent across all three call sites. The original
  code used `&result.PageInfo` (pointer), but Pager copies the struct
  fields internally -- identical behavior, zero allocation.

- **RepoRow/ListRow live in `internal/domain`, not `internal/format`.** The
  plan specified `format/rows.go`, but placing view-model types in `domain`
  respects the invariant that `tui` never imports `format`. Since both
  `format` and `tui` consume these types, `domain` is the correct home.
  Constructor functions (`RepoRowFromDomain`, `ListRowFromDomain`) live in
  `format/rows.go` since they use `humanize` helpers for formatting.

- **RetryService wraps cache, not inner service.** The production stack is
  `lazyService -> RetryService -> cacheService -> diskCacheService ->
  graphQLService`. Retry at the cache boundary means cache hits skip retry
  entirely (idempotent reads served from cache don't need retry), and
  transient errors that make it past cache into the GraphQL call are
  retried before bubbling up.

- **normalizeError applied at DoWithContext boundaries, not at Service
  method returns.** Each GraphQL call in `graphql_service.go` wraps errors
  with `"GitHub GraphQL request failed: %w"` before returning.
  `normalizeError` is applied inside the fetch closures (Pager callbacks)
  so that the typed error information is available throughout the call
  chain, not just at the top-level return.

- **The `graphql_service.go` file still has both static queries and the
  service methods.** A prior iteration had split queries into
  `graphql_queries.go`, but the current structure keeps them colocated.
  This is fine at the current scale -- the queries and their consuming
  methods are tightly coupled.
