# Contributing

## Development setup

```bash
git clone https://github.com/apayne185/en16931-toolkit
cd en16931-toolkit
go build ./...
go test ./...
```

Go 1.22+ required. No external dependencies to install — the toolkit uses the standard library only.

## Before opening a PR

Run the same checks CI runs:

```bash
go vet ./...
go test -race -count=1 ./...
golangci-lint run ./...   # https://golangci-lint.run/usage/install/
```

`.golangci.yml` at the repo root pins the enabled linters (errcheck, govet, staticcheck, unused). CI (`.github/workflows/ci.yml`) runs vet, lint, race-enabled tests with coverage, and a build on every push and PR.

## Branching

- One branch per change, branched from `main`.
- Keep PRs focused: a bug fix or a feature, not both. Split unrelated cleanups into their own PR.
- Rebase onto `main` before opening a PR if your branch is behind — this avoids CI failing on lint/test issues that were already fixed elsewhere.

## Commit messages

Imperative mood, single-line summary under ~70 characters, with an optional body explaining *why* when the change isn't self-evident from the diff:

```
Add IBAN checksum validation to BR-29 (ISO 13616-1 mod-97)
```

## Adding a business rule

Business rules live in `internal/validate/validator.go`, grouped by the EN 16931 section they check (structural, lines, totals, VAT breakdown). Each rule:

1. Has a table-driven test in `internal/validate/validator_test.go` — either a JSON fixture under `testdata/` for rules that need a realistic invoice shape, or an inline case in `TestValidate_InlineEdgeCases` for rules that are awkward to express as a full invoice.
2. Is documented in the rule table in `README.md`.
3. Uses the rule's official EN 16931 code (`BR-*`, `BR-CO-*`) as-is — don't invent new codes for sub-cases the spec doesn't distinguish.

## Adding a country CIUS

Country-specific extensions (like `internal/es` for Spain's Veri\*Factu) should:

- Live in their own package under `internal/`, named for the country.
- Call `validate.Validate` first, then append country-specific checks — never duplicate or bypass the base EN 16931 rules.
- Not be imported by `internal/validate`, `internal/ubl`, or any other country package — the dependency only flows one way, from country package to base validator.

## Reporting bugs

Open an issue with the invoice JSON that triggers the problem (redact real business data) and the expected vs. actual output. If it's a validation rule producing a wrong result, cite the specific BR-* rule and, if you have it, the relevant clause from EN 16931-1:2017.
