# Contributing to keycloak-jit-access

Thanks for your interest in contributing! This project welcomes issues, discussion,
and pull requests.

## Getting started

Prerequisites: Go 1.26+, Docker (for the compose quickstart).

```bash
git clone https://github.com/erreyesarroyo-cloud/keycloak-jit-access.git
cd keycloak-jit-access
go build ./...
go vet ./...
go test ./...
```

To run the full stack locally:

```bash
docker compose up --build
```

## Development workflow

1. Open an issue describing the bug or feature before large changes, so we can agree
   on direction.
2. Fork and create a topic branch (`git checkout -b fix/short-description`).
3. Keep changes focused; one logical change per PR.
4. Run `go build ./...`, `go vet ./...`, and `go test ./...` before pushing.
5. Use clear, imperative commit messages (e.g. "Add early-release audit entry").
6. Open a PR against `main` and fill in the description of what and why.

## Code style

- Standard Go formatting: run `gofmt`/`go fmt ./...`.
- Prefer small, testable functions in `internal/`.
- Any change touching grant/revoke or auth must include an audit-ledger entry.

## Reporting security issues

Please do **not** file public issues for vulnerabilities. See [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions will be licensed under the
[Apache License 2.0](LICENSE).
