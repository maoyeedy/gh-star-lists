## Code Review Checklist

- [ ] New flags added to `Parse`? Update `validateSort`, `validateFilters`, help text, and usage text.
- [ ] New filter key? Add constant to `FilterKey*` block. Add to `validateFilters` switch. Handle in `filterStarLists` and/or `filterRepositories`.
- [ ] New output mode? Must be handled in both `WriteStarListsWithOptions` and `WriteRepositoriesWithOptions` dispatch switches. Add to `SelectOutputMode` validation.
- [ ] New GraphQL query? Must paginate with cursor, accept `$endCursor` and `$first`, check `HasNextPage`.
- [ ] New test asserts on stdout? Must set `Now` in `Options` for deterministic timestamps. Use `testOutputOptions` helper in `run_test.go`.
- [ ] Test uses `errWriter`? Duplicate type defined in both `command_test` and `format_test` packages - this is normal Go isolation.
- [ ] New service feature? Add to `Service` interface, update `cacheService` (both methods), update all `fakeService` implementations in tests.
- [ ] `make check` passes (goimports, ascii-check, vet, test, build).
