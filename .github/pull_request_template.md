## What and why

<!-- What does this change, and why? Link an issue if there is one. -->

## Checklist

- [ ] `go vet ./...`, `go test -race -count=1 ./...`, and `golangci-lint run ./...` pass locally
- [ ] New/changed business rules have a test (fixture in `internal/validate/testdata/` or a case in `TestValidate_InlineEdgeCases`) and are documented in the README's rule table
- [ ] Branch is rebased onto `main` (avoids failing CI on issues already fixed elsewhere)

## Test plan

<!-- How did you verify this? Commands run, manual testing done, edge cases considered. -->
