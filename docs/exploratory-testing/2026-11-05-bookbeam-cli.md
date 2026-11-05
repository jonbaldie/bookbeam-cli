# Exploratory testing report for BookBeam CLI

Date: 2026-11-05 UTC. Build: `origin/main` at `426bf4e4933c6120ef57880a098ab1ee18be47dc`, Go 1.26.8, Linux amd64. The supported interface was the built `bookbeam` binary.

Three journeys used a local HTTP fixture with synthetic data. No real account, API token, subscriber data, or production service was used. The confirmed CSV data-loss case is already covered by [issue #46](https://github.com/jonbaldie/bookbeam-cli/issues/46). The [new reproduction comment](https://github.com/jonbaldie/bookbeam-cli/issues/46#issuecomment-6647142667) adds observed evidence. No product code was changed.

## Setup and evidence

A worktree started at `origin/main`. `GOFLAGS=-mod=readonly go mod download` and `go mod verify` used the checked-in `go.mod` and `go.sum`. Module cache, build cache, temporary files, config, and binary stayed in the worktree. Both module files retained their checksums. See [setup.log](2026-11-05-evidence/setup.log).

The [replay script](2026-11-05-evidence/replay.py) starts an in-process fixture on an ephemeral loopback port, runs the binary, and stops and joins the server in `finally`. It owns its temporary directory and uses a fake token. The [session log](2026-11-05-evidence/session.log) records the original commands, outputs, requests, and state checks. After review, the script replaced a threaded `preexec_fn` with a separate process wrapper and added state assertions. The [replay check](2026-11-05-evidence/replay-check.log) records the same results with that revision. To replay on Linux, create `.explore/` at the repository root, build the CLI to `.explore/bookbeam`, and run `python3 -I docs/exploratory-testing/2026-11-05-evidence/replay.py`. The failed-write experiment uses `RLIMIT_FSIZE` and `SIGXFSZ`.

## Journeys

### Direct login, identity, and logout

Goal: save a token, use it for identity, and remove it on logout, as described in the README.

- `auth login --token fake-token --json` succeeded. The isolated config contained the fake token and had mode `0600`.
- `whoami --json` returned the fixture identity. The fixture received `Bearer fake-token`.
- `auth logout --json` succeeded, and the config no longer contained a token. A later `whoami` failed before sending a request.

The browser device flow and real service authentication were not exercised. No new bug was found in this journey.

### Subscriber CSV export

Goal: export subscriber records to a file or stdout, and retain a previous complete export if a replacement fails. The README documents the file export; `-o -` is documented in command help and implemented by [PR #99](https://github.com/jonbaldie/bookbeam-cli/pull/99).

- A JSON file export succeeded. The file matched the fixture's 348 bytes and the JSON reported that byte count.
- `-o -` wrote the same CSV to stdout and its confirmation to stderr.
- A process file-size limit forced a local write failure. In two fresh replays the command returned an error, but replaced the previous complete CSV with the first 64 bytes of the new response. This confirms the failed-write case already covered by [#46](https://github.com/jonbaldie/bookbeam-cli/issues/46), whose acceptance criteria require an existing destination to remain unchanged.

The fixture returned CSV bytes directly. It does not establish how the production service sanitizes CSV fields or whether the service can generate a particular export size. The simulated limit establishes a write-failure case, not an observed production disk-full event.

### Book file download

Goal: download a book to disk, and keep an existing complete copy if a transfer fails. This follows the README download command and the safety expectation in [#86](https://github.com/jonbaldie/bookbeam-cli/issues/86).

- A normal download used the fixture's `Content-Disposition` filename and wrote the expected 19 bytes. JSON reported the same size.
- The fixture then advertised 100 bytes but closed after seven. In two replays the CLI returned `unexpected EOF`, preserved the prior `keep.epub` bytes, and left no `.bookbeam-download-*` file in the test directory.

This checks a short response, not signal interruption, large files, upload progress, or every failure mode. It does not reopen #86.

## Confirmed finding

**Existing issue [#46](https://github.com/jonbaldie/bookbeam-cli/issues/46): a failed CSV write destroys the previous export.** A user refreshing a local subscriber backup can lose the prior complete file and retain a partial replacement. The replay starts with `previous complete export\n` in a destination file, then sets the CLI process's `RLIMIT_FSIZE` to 64 bytes with `SIGXFSZ` ignored. Run `bookbeam downloaders export 7 -o keep.csv` against the fixture returning 348 bytes. The expected result is an error with the previous file unchanged. The actual result is exit 1 with `file too large`, and the destination contains the first 64 bytes of the new response. Two independent fresh destinations produced the same result. The [session log](2026-11-05-evidence/session.log) records both attempts and the [replay script](2026-11-05-evidence/replay.py) contains the exact fixture and steps.

## Other observations and limits

- The first `go test ./...` run failed because the inherited `umask 077` made a test-created CSV mode `0600`, while `TestNtdDownloadersExportWritesCSV` expects `0644`. This is a test environment assumption, not a confirmed product bug. See [checks.log](2026-11-05-evidence/checks.log). With `umask 022`, `go test ./...` and `go vet ./...` passed. See [checks-umask022.log](2026-11-05-evidence/checks-umask022.log).
- Local filesystem write errors print a usage block along with the error. Error presentation is already tracked in [#44](https://github.com/jonbaldie/bookbeam-cli/issues/44). No duplicate was filed.
- No live credentials were available in the environment or the default config path. Live account, browser login, catalog mutations, newsletter integration, upload, billing, Windows, and shell completion remain unexplored.
- No unresolved candidate remains from these journeys. The CSV finding is confirmed but already tracked. The short-response download candidate was rejected by the two successful preservation checks.

## Teardown

The script stopped and joined its fixture, closed the listening socket, and removed the temporary data directory. The session log records `fixture stopped= True`; a subsequent socket check found no listener on the recorded port. No browser, container, Docker image, or Docker volume was started by this pass. The worktree and its local dependency caches and binary are disposable and are removed after publication.
