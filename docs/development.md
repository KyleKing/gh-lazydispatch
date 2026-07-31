# Development

Setup, tasks, building, testing, test safety, demo recording, and the release process live in [CONTRIBUTING.md](../CONTRIBUTING.md), which the copier template re-renders on every update. This page holds the one routine that file does not cover yet, and it belongs in the template once it has a section for it.

## Updating dependencies

Check [go.dev/dl](https://go.dev/dl/) for the current Go release, raise the version in `go.mod`, then:

```bash
go get -u ./... && go mod tidy && go test ./...
```

GitHub Actions pins live in `.github/workflows/*.yml`. Their release pages:

- [actions/checkout](https://github.com/actions/checkout/releases)
- [actions/setup-go](https://github.com/actions/setup-go/releases)
- [golangci-lint](https://github.com/golangci/golangci-lint/releases)
- [goreleaser-action](https://github.com/goreleaser/goreleaser-action/releases)

Then verify:

```bash
mise run ci
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | grep total
```
