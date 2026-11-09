# State-machine coverage

Reviewed source commit `14b3363649eb5f7889310bfc52979fee6c015176`. Exact file hashes are in [source.json](source.json). Each model directory contains its source map, finite bounds, configuration manifest, and real-code replay tests.

| Inventory | Model | Covered source and functions |
|---|---|---|
| S1 | auth/AuthPoll | `cmd/auth.go`, device login and `pollDeviceToken` |
| S2, S3 | auth/Credentials | `pkg/config/config.go`, resolution, `StoreToken`, persistence, effective overrides. `cmd/root.go` client snapshot. |
| S4 | auth/Readiness | `cmd/root.go` construction and pre-run, `cmd/authentication.go` inherited authentication gates. `main.go` command termination. |
| F1 | mutations/FieldEdits | `cmd/field_edit.go`, flag validation, JSON and form encoding, existing-value merge |
| S5, S6 | mutations/ReadMergeWrite | `cmd/links.go` link update and lookup. `cmd/projects.go` newsletter routing update. |
| S7 | mutations/DeleteGate | `pkg/output/printer.go` `Proceed`. Delete handlers in projects, files, and links. |
| S8 | io/HTTP | `pkg/client/client.go` request dispatch, status boundary, body ownership, short-call and transfer deadlines |
| S9 | io/Multipart | `pkg/client/client.go` `PostMultipartWithProgress`, cancellation, and bounded progress delivery. Preparation consumers in files and projects. |
| S10 | io/Download | `cmd/files.go` download and `cmd/downloaders.go` CSV export |
| S11 | io/Output | `pkg/output/printer.go` `Display`, `encode`, `Launch`. Sanitization preparation in `sanitize.go`. |
| Remote projections | mutations/RemoteRequests | `cmd/newsletter.go`, `cmd/billing.go`, catalog commands. Request/report ordering and representative input projections only. |
| Repository automation | mutations/Workflow | `.github/workflows/ci.yml` and `release.yml`. Abstract fail-stop sequencing and five release packages. |

## Verification limits

The models exhaust their finite abstract transition graphs. Their source maps define the atomic actions and assumptions. Code replay tests exercise actual Go functions with generated finite input sets and deterministic HTTP fixtures. They establish representative correspondence, rather than a proof that Go refines TLA+.

Authentication uses two polls and a two-second abstract expiry. Token values reduce to absent, empty, and nonempty. Credential values and filesystem outcomes are finite representatives. One read, one optional competing edit, and one write bound each partial-update invocation. Resource models cover one request or transfer and independently scheduled multipart cleanup. Workflow models cover three CI platforms and five sequential release packages.

The tests do not contact the production service. Remote persistence, token validity, deletion cascades, payment completion, and entitlement transitions remain server-owned. The stale-update finding establishes an unversioned stale payload and reproduces data loss against a last-write-wins HTTP fixture. It does not establish the production server's concurrency policy.

No liveness proof assumes eventual network completion or goroutine scheduling. Filesystem fault outcomes abstract platform-specific errors. Filesystem durability, arbitrary byte counts, unbounded backoff, dependency internals, and every possible CLI flag combination are outside the proof.

Completion dispatch, ID validation, response decoding, telemetry filtering, pagination policy, and pure formatting are not additional state machines. The source guard tracks their files so changes still require a coverage review. CI YAML execution itself is not formally verified by the abstract Workflow model.

## Read results

The runner writes raw TLC logs, Go baseline and mutation logs, and `summary.json` into its printed result directory. A successful runner means that baseline properties held and every deliberate mutant and recorded finding produced its expected named violation. It does not mean that all desired user properties hold.

Use `/statechart` to read every TLA+ result. Recheck any new violation against real code. Do not convert an existing finding into a passing baseline by weakening its rule.
