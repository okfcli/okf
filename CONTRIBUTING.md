# Contributing to okf

Thank you for your interest in contributing. Contributions are very welcome,
whether you are fixing a typo, filing a bug, adding a command, or reshaping a
subsystem. This guide explains how to get set up, what we expect in a change,
and how to get it merged.

`okf` is a Go CLI toolkit for the [Open Knowledge Format (OKF)](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md).
It is designed to be one static binary, fast, and driven by both humans and AI
agents. Keep that spirit in mind as you contribute.

## Code of conduct

This project is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By
participating, you are expected to uphold it. Report unacceptable behavior to
accounts+conduct@akeemjenkins.com.

## AI-assisted contributions are welcome

We build `okf` to be agentic-first, so it is only fitting that we welcome
contributions written with the help of AI tools (Claude Code, Copilot, Cursor,
and others). There is no stigma here. A good pull request is a good pull request
regardless of how it was produced.

We ask a few things when AI helped write your change:

- **You are the author and you are accountable.** Read, understand, and stand
  behind every line you submit. "The model wrote it" is not a defense for a bug
  or a licensing problem. If a reviewer asks why a line exists, you should be
  able to answer.
- **Run everything locally before you open the PR.** AI-generated code that was
  never built or tested wastes reviewer time. See [Before you submit](#before-you-submit).
- **Do not paste in code you do not have the right to contribute.** Only submit
  code you wrote or are licensed to contribute under Apache 2.0 (see
  [Licensing and sign-off](#licensing-and-sign-off)). Do not let a tool
  reproduce large verbatim chunks of someone else's licensed code.
- **Keep the diff reviewable.** Do not submit a sprawling machine-generated
  refactor with unrelated churn. Tight, focused, well-explained diffs get merged
  faster than large ones no matter who or what wrote them.
- **Disclose substantial AI authorship.** A short note in the PR description
  ("drafted with Claude Code, reviewed and tested by me") is appreciated and
  builds trust. It is not required for small changes.

The same quality bar applies to all contributions. AI assistance does not lower
it and does not raise it.

## Getting started

### Prerequisites

- **Go 1.26 or newer** (see `go.mod` for the exact version the module targets).
- **make** for the convenience targets.
- **git**.
- Optional but recommended for matching CI locally:
  [`golangci-lint`](https://golangci-lint.run/) and
  [`govulncheck`](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck).

### Set up the repo

```bash
git clone https://github.com/okfcli/okf.git
cd okf
go mod download
make build     # produces ./bin/okf
make test      # runs the suite
```

Run the binary you just built:

```bash
./bin/okf schema
./bin/okf init /tmp/demo-bundle
./bin/okf validate /tmp/demo-bundle
```

## Development workflow

1. **Open or find an issue first for anything non-trivial.** For typos, small
   docs fixes, and obvious bugs, a direct PR is fine. For new commands, flags,
   output-format changes, or refactors, open an issue so we can agree on the
   approach before you invest time.
2. **Branch from `main`.** Use a short, descriptive branch name such as
   `fix-broken-link-detection` or `add-serve-command`.
3. **Make focused commits.** One logical change per commit where practical.
4. **Write or update tests** for any behavior you add or change.
5. **Run the full local check** (see below) and make it green.
6. **Open a pull request** against `main` with a clear description.

### Common make targets

| Target | What it does |
|--------|--------------|
| `make build` | Build the `okf` binary into `./bin` |
| `make test` | Run the test suite |
| `make test-race` | Run tests with the race detector |
| `make vet` | Run `go vet` |
| `make check` | `vet` + `test-race` (the pre-push gate) |
| `make cover` | Run tests with a coverage profile |
| `make cover-html` | Generate an HTML coverage report |
| `make tidy` | Run `go mod tidy` |

## Coding standards

- **Format with `gofmt`.** All code must be gofmt-clean. Most editors do this on
  save; otherwise run `gofmt -w .`.
- **Pass the linters.** CI runs `golangci-lint` with the config in
  `.golangci.yml` (govet, staticcheck, errcheck, ineffassign, unused, revive,
  gosec, misspell). Run `golangci-lint run` locally to catch findings early.
- **Handle errors explicitly.** Return structured errors that fit the existing
  error-envelope model. Do not swallow errors silently.
- **Keep the CLI agentic-first.** These are load-bearing invariants, not style
  preferences:
  - Commands emit structured JSON on stdout by default. Diagnostics go to
    stderr.
  - Errors emit the `{"error": {"kind", "code", "reason", "message"}}` envelope
    on stdout with a stable exit code.
  - Exit codes are stable and documented in the README. Do not change their
    meaning without discussion.
  - New commands must be discoverable through `okf schema`.
- **No new runtime dependencies without discussion.** A core value of `okf` is
  the single static binary with a tiny dependency tree. If your change needs a
  new module, explain why in the issue or PR first.
- **Match the surrounding code.** Follow the naming, structure, and idioms
  already in the package you are editing.

## Testing

- The project uses standard Go testing (`go test ./...`), much of it written
  test-first. New behavior needs test coverage.
- Bug fixes should include a regression test that fails before your fix and
  passes after.
- Put fixtures under `testdata/` following the existing layout.
- Run the race detector before you push: `make test-race`.

## Before you submit

Run the same checks CI will run. All of these must pass:

```bash
gofmt -l .              # should print nothing
go vet ./...
go build ./...
make test-race
golangci-lint run       # if installed
govulncheck ./...       # if installed
```

The CI pipeline (`.github/workflows/ci.yml`) builds and tests on Linux, macOS,
and Windows, runs `golangci-lint` and `govulncheck`, and uploads a build
artifact. Matching it locally is the fastest path to a green PR.

## Pull request guidelines

- **Title:** a concise summary of the change.
- **Description:** what changed and why. Link the issue it addresses with
  `Closes #123` where applicable.
- **Scope:** keep each PR to one coherent change. Split unrelated changes into
  separate PRs.
- **Docs:** update the README, `okf schema` output, and any relevant docs when
  you change user-facing behavior.
- **Green CI:** all checks must pass before review. A maintainer will review,
  possibly request changes, and merge once it is ready.
- **Be responsive** to review feedback. We aim to be prompt and constructive in
  return.

## Reporting bugs

Open an issue with:

- What you ran (the exact command and, if relevant, a minimal bundle).
- What you expected to happen.
- What actually happened, including the full JSON error envelope or output.
- Your `okf version`, OS, and Go version.

A minimal reproducible example is the single most helpful thing you can provide.

## Requesting features

Open an issue describing the problem you want to solve, not just the solution
you have in mind. Explain the use case, especially if it improves the experience
for AI agents driving the CLI. We are happy to discuss design before you write
code.

## Security issues

Please do not open a public issue for security vulnerabilities. Instead, email
accounts+conduct@akeemjenkins.com with details and we will coordinate a fix and
disclosure privately.

## Licensing and sign-off

`okf` is licensed under [Apache 2.0](LICENSE). By submitting a contribution, you
agree that your contribution is licensed under the same terms, and you certify
that you have the right to submit it (in the spirit of the
[Developer Certificate of Origin](https://developercertificate.org/)). Only
submit code you wrote or are otherwise licensed to contribute.

## Questions

If anything here is unclear, open an issue with the `question` label or start a
discussion. We would rather answer a question early than review a PR that went
the wrong direction. Thank you for helping make `okf` better.
