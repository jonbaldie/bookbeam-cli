# Exploratory Testing Report: bookbeam-cli

**Date:** 2026-10-10 (UTC)  
**Build:** `main` at `c4bac1f37247c7930b820d1396a6a74cd3f5d63f`, Go 1.26.3, macOS arm64  
**Interface:** the built `bookbeam` binary, using documented commands and flags  
**Result:** three journeys exercised; one new confirmed bug filed as [#73](https://github.com/jonbaldie/bookbeam-cli/issues/73). No product changes made.

## Setup and evidence

The checkout was fast-forwarded from `36154bb` to the remote default branch before testing. Dependencies were downloaded with `GOFLAGS=-mod=readonly go mod download`, using `go.mod`/`go.sum`. Their SHA-256 checksums stayed unchanged. `GOMODCACHE` and the binary were inside `.exploratory-run/` in this workspace; the supplied `TMPDIR` and shared `GOCACHE` were retained. No dependency or build-output symlinks into another checkout were used.

Live commands targeted `https://bookbeam.app`, using the existing PAT through `BOOKBEAM_TOKEN` and an isolated `BOOKBEAM_CONFIG_DIR`. The real config was read, never edited. `whoami` established readiness. The catalog started with seven projects (IDs 21, 6, 5, 4, 3, 2, 1). Writes were confined to disposable project **27**, link **24**, and file **29**. Auth variations used fake tokens and separate local configs.

Evidence in [`2026-10-10-evidence/`](2026-10-10-evidence/):

- [`setup.log`](2026-10-10-evidence/setup.log): build, revision and unchanged dependency checksums.
- [`session.log`](2026-10-10-evidence/session.log): actions, stdout, stderr, exit codes and outcome checks; PAT/provider credential fields and emails redacted.
- [`run.py`](2026-10-10-evidence/run.py): public-CLI journey driver. **Re-running it creates and deletes one live project.**
- [`permissions-replay.py`](2026-10-10-evidence/permissions-replay.py) and [replay log](2026-10-10-evidence/permissions-replay.log): fake-token reproducer requiring no network.
- [`tests.log`](2026-10-10-evidence/tests.log), [`vet.log`](2026-10-10-evidence/vet.log), and [`tracked-checks.log`](2026-10-10-evidence/tracked-checks.log): original local failures and the disclosed verification workaround.
- [`cleanup.log`](2026-10-10-evidence/cleanup.log): Docker inventory, workspace cleanup, and cleanup failure/recovery diagnostics.

To prepare a replay from the repository root:

```bash
export GOMODCACHE="$PWD/.exploratory-run/modcache"
export GOFLAGS=-mod=readonly
go mod download
go build -o .exploratory-run/bookbeam .
python3 docs/exploratory-testing/2026-10-10-evidence/permissions-replay.py
# Optional live journey replay, using your configured account:
# python3 docs/exploratory-testing/2026-10-10-evidence/run.py
```

## Journeys exercised

### 1. Authenticate, override settings, and log out

**Goal:** a headless user can save a token securely, apply one-off settings without persisting them, then remove saved credentials. Grounded in README authentication/environment-variable documentation and the owner-only `config.save` contract.

- Fresh unauthenticated `whoami` exited 1 with the correct `bookbeam auth login` hint.
- Direct login returned valid `--json` with `logged_in: true`; independently reading the config showed the supplied fake token. New directory/file modes were `0700`/`0600`.
- One-off environment host/token overrides targeted a deliberately unreachable localhost endpoint, failed as expected, and left the stored config byte-identical.
- Logout returned valid JSON with `logged_in: false` and removed the stored token while preserving the host.
- Restoring the config file to `0644`, then logging in again, exposed the confirmed permissions bug. All these auth steps ran twice in separate directories.

### 2. Create and edit a project and its signup link

**Goal:** an author can edit individual fields without losing untouched copy, explicitly clear fields, cancel deletion, and then delete. Grounded in README project commands and `links update --clear-consent` help.

- Created project 27 with a title and description. Renamed it, then fetched it in a separate invocation: title changed; description survived.
- `--clear-description` persisted a null description. Combining it with `--description Conflict` exited 1; a fresh fetch proved the state stayed cleared.
- Created link 24 with title and consent. Title-only update preserved consent; `--clear-consent` persisted null.
- Answering `n` to link deletion left it present. Forced deletion removed it from a subsequent list.
- Answering `n` to project deletion also left it present. Final forced deletion returned success, followed by GET 404 and the original seven-project catalog.

**Result:** attempted lifecycle and corrective/cancellation variations passed.

### 3. Round-trip a book file and export subscribers

**Goal:** upload a book, retrieve identical bytes under explicit and negotiated filenames, then export a usable CSV. Grounded in README file/download/export commands and CLI JSON metadata.

- Uploaded a 1,230-byte EPUB fixture named `two words.epub`. A fresh file list showed the original filename.
- Downloaded with `-o downloaded.epub`: bytes matched the upload exactly, and JSON `bytes_written` matched file size.
- Removed the local upload fixture and downloaded again without `-o`: the original filename was negotiated and the bytes still matched.
- Cancelled file deletion; a fresh list proved it survived. Forced deletion removed it.
- The disposable project had no downloaders. Explicit-output export saved a 35-byte header-only CSV (`Email, Signed Up At, Source Link`); JSON metadata matched the actual file. Default-output export survived reopening and matched those bytes.

**Result:** attempted small-file round trips, cancellation and empty-export variation passed. Populated subscriber export was not exercised.

## Confirmed bug: existing config permissions are not secured on login

**Issue:** [#73 — auth login leaves PAT readable in pre-existing 0644 config.json](https://github.com/jonbaldie/bookbeam-cli/issues/73).

**Impact:** a config created manually or restored with ordinary `0755` directory / `0644` file modes can receive a PAT while retaining other-user traversal/read permission bits. Actual cross-user access also depends on ancestor permissions and ACLs; it was not attempted.

**Starting state:** a separate directory containing `config.json` with only `{"host":"https://bookbeam.app"}`; directory mode `0755`, file mode `0644`; no environment host/token overrides.

**Minimal replay:** set `BOOKBEAM_CONFIG_DIR` to that directory and run:

```bash
bookbeam auth login --token fake-permission-replay-token --json
```

Then inspect file permissions and saved content. The [executable replay](2026-10-10-evidence/permissions-replay.py) creates that starting state independently twice and cleans it up.

**Expected:** owner-only `0600` credentials, matching fresh login and `config.save`'s stated contract.

**Actual:** exit 0, `logged_in: true`, token saved, directory still `0755`, file still `0644`. Repeated **2/2** focused replays. The initial discovery also repeated **2/2**, with a restrictive `0700` parent containing the exposure.

**Explanation after observing the failure:** `pkg/config/config.go` uses `os.MkdirAll(..., 0700)` and `os.WriteFile(..., 0600)`. Those creation modes do not tighten existing paths. This pass did not implement a fix.

## Other observations and candidate classification

- **Known failure, not duplicated:** the deleted-project GET printed a usage block before its API 404 error, including under `--json`. Evidence was added to existing [#6](https://github.com/jonbaldie/bookbeam-cli/issues/6#issuecomment-6092368869). Structured error handling is also already tracked in #44.
- **Rejected as a product bug:** local `go test ./...` and `go vet ./...` failed because the pre-existing, untracked `cmd/fuzz_test.go` references the removed `buildProjectUpdatePayload` helper. This file is not part of the tested remote revision. It was neither edited nor committed. A Go overlay replaced only that file with `package cmd` for verification of tracked code; tests and vet then passed. Original diagnostics remain in the evidence.
- **Usability observation:** deletion cancellation exits 0 and prints `Cancelled.`. The follow-up reads were necessary to distinguish cancellation from deletion; no improvement is proposed here.
- No unresolved failure candidates remain from the attempted journeys.

## Limitations and unexplored areas

- One production account/team; live writes limited to the disposable project. No live newsletter provider, webhook, billing, or existing project settings were changed.
- Browser/device-flow authorization was not exercised: it requires human approval and this was an AFK pass.
- Public signup-page browsing, actual reader signup, cover-image transfers, newsletter routing, telemetry pagination, populated subscriber exports, large/slow/interrupted transfers, and Windows/Linux permissions were not tested in this pass.
- Most successful command results used `--json`; human output was exercised for login, unauthenticated/errors and cancellation prompts. This is not a full table-output audit.
- Permission replay changed filesystem modes deliberately and used fake tokens; it did not test another OS user's access.

## Cleanup

- Link 24, file 29 and project 27 deleted. A fresh project GET returned 404; catalog IDs and total returned to their starting values.
- Real config stayed byte-identical throughout the live pass.
- All run-owned binary, module-cache, fake-token config, EPUB/download, CSV and overlay scratch state removed. Plain `rm` initially failed on Go's read-only module directories; `go clean -modcache` removed the local cache, and the final removal succeeded. Diagnostics are preserved.
- **Docker containers/images/volumes created: 0/0/0.** Docker was used only for inventory; before/after inventories matched. No pre-existing Docker resources were removed.
- The existing untracked `cmd/fuzz_test.go` remains untouched. Report, replay drivers and redacted evidence are retained under `docs/`.
