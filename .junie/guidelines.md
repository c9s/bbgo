BBGO Project Development Guidelines

Audience: Advanced Go developers contributing to this repository.

This document summarizes project-specific build, configuration, testing, and development practices that are not obvious from general Go conventions. Commands assume repository root as working directory.

1) Build and configuration

Core binaries and build tags
- Main binary: ./cmd/bbgo
- Build tags are significant:
  - web: embed the React dashboard and backtest report assets into the binary.
  - release: enables release build flags and version stamping.
  - dnum: switches the project to high-precision decimal math in selected code paths; some tests are excluded under dnum via build tags.

Make targets (validated)
- Slim native build (no embedded web assets, fastest when you don’t need dashboard):
  make bbgo-slim
  Produces build/bbgo/bbgo-slim

- Full native build (includes dashboard assets):
  make bbgo
  Produces build/bbgo/bbgo
  Notes: Requires embedded assets (see “Asset embedding” below) or pre-generated assets in pkg/server/assets.go and pkg/backtest/assets.go.

- Cross-compile presets (useful in CI/release):
  make bbgo-linux         # linux/amd64 + linux/arm64 with web,release
  make bbgo-darwin        # darwin/amd64 + darwin/arm64 with web,release
  make bbgo-slim-linux    # linux variants without web tag
  make bbgo-slim-darwin   # darwin variants without web tag
  DNUM variants exist with -dnum suffix in targets (e.g., bbgo-dnum-linux).

- Version stamping and release tooling:
  make version            # bumps version files and tags (expects release note file)
  make dev-version        # updates dev build version file

Asset embedding (web dashboard and backtest report)
- To include the web dashboard in the binary, assets must be embedded via pkg/server/assets.go and pkg/backtest/assets.go. The Makefile will generate these using the utils/embed helper after building frontend artifacts.
- Minimal path to regenerate assets locally:
  1) Build frontend:
     cd apps/frontend && yarn install && yarn export
  2) Build backtest report:
     cd apps/backtest-report && yarn install && yarn build && yarn export
  3) Generate Go embed files (from repo root):
     go run ./utils/embed -package server -tag web -output pkg/server/assets.go apps/frontend/out
     go run ./utils/embed -package backtest -tag web -output pkg/backtest/assets.go apps/backtest-report/out
  Or simply run:
     make static

Notes
- If you do not need the dashboard while iterating, prefer bbgo-slim targets to avoid the frontend toolchain.
- go.mod specifies Go 1.23; use a matching toolchain (Go 1.23 or newer compatible toolchain).

Database migrations and code generation
- SQL migrations are managed via rockhopper; compiled Go migration packages can be refreshed with:
  make migrations
  This uses rockhopper configs (rockhopper_mysql.yaml, rockhopper_sqlite.yaml) to regenerate pkg/migrations/*.

- gRPC/protobuf interfaces are in pkg/pb/*.proto. Regenerate with:
  make grpc-go            # requires protoc and plugins; see make install-grpc-tools
  make grpc               # go + python

Docker image
- The Docker build is wired to reuse Go module cache via _mod:
  make docker
  To push:
  make docker-push DOCKER_TAG=vX.Y.Z

2) Testing

Overview
- Standard Go test tooling applies. Many packages have pure unit tests that do not require credentials. Some integration tests may rely on exchange sessions and environment variables (e.g., BINANCE_* or MAX_* based on envVarPrefix in configs). For local development, prefer running tests in packages that are dependency-free, or mock external deps.

Running tests
- Run all tests (can be heavy):
  go test ./...

- Run tests in a specific package (recommended during iteration):
  go test ./pkg/bbgo -v -run TestLoadConfig

- Race detector and coverage:
  go test ./... -race
  go test ./... -cover -coverprofile=coverage.txt

Build tags in tests
- Some tests are guarded by build constraints. Example: pkg/bbgo/config_test.go has //go:build !dnum at the top, meaning it runs only without the dnum tag.
  To run with dnum (fewer tests may match):
    go test -tags dnum ./...

Environment-based tests
- Exchange sessions are configured by envVarPrefix (see Config). If a test or example requires live credentials, set env vars accordingly, e.g.:
  BINANCE_API_KEY=... BINANCE_API_SECRET=...
  MAX_API_KEY=... MAX_API_SECRET=...
  However, most unit tests in this repository avoid hitting live services; prefer running targeted packages.

Demonstration: creating and running a simple test (validated)
We verified the test workflow end-to-end using a minimal, dependency-free test to avoid external side effects:

- Created file (temporary during validation, now removed): pkg/devdocdemo/demo_test.go
  --- contents ---
  package devdocdemo
  import "testing"
  func TestDemo(t *testing.T) {
    if 2+2 != 4 {
      t.Fatalf("math broken")
    }
  }

- Ran the test:
  go test ./pkg/devdocdemo -v

- Observed output:
  === RUN   TestDemo
  --- PASS: TestDemo (0.00s)
  PASS
  ok  	github.com/c9s/bbgo/pkg/devdocdemo	0.481s

- Cleaned up: removed pkg/devdocdemo after verification to keep the tree clean. You can replicate this pattern in any package you’re modifying.

Guidelines for adding new tests
- Place *_test.go files alongside the package under test. Prefer pure unit tests with clear seams to mock external APIs.
- Use testify where helpful (github.com/stretchr/testify) — already in go.mod and used widely (e.g., assert in pkg/bbgo/config_test.go).
- For tests requiring yaml fixtures, put them under a testdata/ directory next to the tests (standard go tooling will ignore it for packages).
- Avoid requiring network or credentials unless the test is explicitly marked or skipped behind an env guard. Example pattern:
  if os.Getenv("INTEGRATION") == "" { t.Skip("integration test") }

3) Additional development information

Code style and tooling
- Use gofmt and go vet. The repository already formats generated files via gofmt in Makefile (e.g., dev version generation). Adopt `gofmt -s -w` on touched files when needed.
- Keep build tags consistent. If adding dnum-specific code paths, mirror test constraints with //go:build dnum or //go:build !dnum as appropriate.
- Respect module boundaries: this is a multi-package repo. Prefer small, testable packages.

Versioning and release notes
- Release flow relies on generated version files (pkg/version/*) and a release note under doc/release/<VERSION>.md. See make version.

Frontend updates
- Any change to apps/frontend or apps/backtest-report requires rebuilding and re-embedding assets before producing a full (web-tagged) binary.

Configuration examples
- There are multiple YAML examples at repo root and under config/ and examples/. For strategy development, review docs under doc/, strategy code under pkg/strategy/*, and the sample configs in the repository root (e.g., xmaker-*.yaml).

Practical tips
- Prefer bbgo-slim builds for local strategy iteration to avoid the asset pipeline. Switch to full web,release build only when you need to validate the dashboard or to package a distributable binary.
- When touching migrations or protobufs, run the corresponding Make targets and commit the generated files to keep CI green.
- When adding strategies, wire them via bbgo.RegisterStrategy and provide minimal config tests similar to pkg/bbgo/config_test.go to keep backward compatibility verifiable.

Maintenance note
- This guidelines file is intended to be project-specific and concise for experienced contributors. If you add new tooling or workflows (e.g., new Make targets, linter configs), update this document accordingly.
