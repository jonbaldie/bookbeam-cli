# Exploratory Testing Report: bookbeam-cli

**Date:** 2026-10-24 (UTC)
**Build:** `main` at `3810870eabf43ffba2ceeed01f7eabbcaacaad74`, Go 1.26.3, macOS arm64
**Interface:** the built `bookbeam` binary, run with documented commands and flags
**Result:** three journeys exercised. Three new confirmed bugs filed: [#86](https://github.com/jonbaldie/bookbeam-cli/issues/86), [#87](https://github.com/jonbaldie/bookbeam-cli/issues/87), [#88](https://github.com/jonbaldie/bookbeam-cli/issues/88). No product changes made.

## Setup and evidence

Dependencies were downloaded with `GOFLAGS=-mod=readonly go mod download` from `go.mod`/`go.sum`, then checked with `go mod verify`. The checksums of both files did not change. `GOMODCACHE` and the binary lived in `.exploratory-run/` inside this workspace. The supplied `TMPDIR` and the shared `GOCACHE` were used. No dependency or build-output symlinks into another checkout were used.

Live commands targeted `https://bookbeam.app`. The existing PAT was passed through `BOOKBEAM_TOKEN`, with an isolated `BOOKBEAM_CONFIG_DIR`. The real config was read but never edited. `whoami` established readiness: Jonathan Baldie, Jonathan's Team, `https://bookbeam.app`. Starting state: 7 projects (IDs 21, 6, 5, 4, 3, 2, 1), metrics 7 projects / 12 files / 8 downloads / 1527 views, newsletter provider Mailcoach (configured), webhook `null`. Writes were confined to disposable project **29**, its files **31–34**, link **26**, and the team webhook. The webhook was restored to `null`.

Evidence in [`2026-10-24-evidence/`](2026-10-24-evidence/):

- [`setup.log`](2026-10-24-evidence/setup.log): build, revision, unchanged dependency checksums, starting Docker inventory.
- [`session.log`](2026-10-24-evidence/session.log): actions, stdout, stderr, and exit codes, with the PAT and emails redacted. Seven early entries show `exit=1` because of a driver error: zsh did not word-split the argument string. They are annotated and repeated with the correct argv.
- [`bb.py`](2026-10-24-evidence/bb.py): the public-CLI driver. Usage: `python3 -I bb.py run <config-dir> -- <bookbeam args>`.
- [`interrupt.py`](2026-10-24-evidence/interrupt.py): starts `files download`, sends a signal after a delay, and records the destination file before, at the signal, and after. **It downloads a live file. It needs an existing project/file.**
- [`cleanup.log`](2026-10-24-evidence/cleanup.log): Docker inventory and workspace cleanup.

## Journeys exercised

### 1. Script the catalog with `--json` and page through it

**Goal:** a script gets the same facts as the table: effective host, provider configured, public signup URLs. Paging past the end says so, and does not claim the collection is empty. This is grounded in #79 (derived fields in JSON documents) and the #80 fix.

- `whoami --json` carried `api_host: "https://bookbeam.app"`. `newsletter status --json` carried `configured: true`, with `api_token` masked. The table showed `Configured Yes`.
- `links list 5 --json` carried `public_url` per item, and it matched the table. Under `--host HTTPS://BookBeam.app/`, `public_url` became `https://BookBeam.app/download/…`: scheme lowercased, host case kept, trailing slash dropped. That is a valid URL. Noted, not filed.
- `projects list --page 2`, `downloaders list 5 --page 2`, and `logs --page 99 --limit 3` all printed `Page N is past the last page.` with a footer showing the real total (7, 2, 11). `logs --event no-such-type` printed `No activity logs matched event "no-such-type" on this page.` The #80 fix holds.
- `logs --event signup` filtered the table. `--json` stayed unfiltered, as the flag help states.
- Errors under `--json` (`projects get 999999`, `projects get abc`, unauthenticated `projects list`) printed the usage block on stderr and nothing on stdout. Exit code was 1. That is known: [#6](https://github.com/jonbaldie/bookbeam-cli/issues/6), [#44](https://github.com/jonbaldie/bookbeam-cli/issues/44), [#76](https://github.com/jonbaldie/bookbeam-cli/issues/76).

**Result:** the new JSON fields and the past-the-end messages held. No new bug.

### 2. Upload and download a full-size book

**Goal:** an author uploads a realistically sized book, downloads the same bytes back, and stays safe when a transfer is cut short or the destination is bad. This is grounded in the README Book Files commands ("Upload a file with progress tracking", "Download a book file to disk").

- Created project 29. Uploaded a 1,127-byte EPUB (file 31) and a 26,215,629-byte EPUB (file 32). The large upload took about 15 s.
- The default-name download wrote `big.epub` (about 6 s). `-o explicit.epub --json` reported `bytes_written: 26215629`. Both outputs were byte-identical to the upload (sha256 `4024f4f2…`).
- `-o adir` (a directory) and `-o nosuch/dir/x.epub` failed with clear `failed to create destination file` errors. A missing file id returned 404 before any file was created. Author downloads did not change the `DOWNLOADS` column or the metrics.
- **Interrupting a download (SIGINT/SIGTERM) truncated an existing complete copy and left partial files under the final name.** See the confirmed bug [#86](https://github.com/jonbaldie/bookbeam-cli/issues/86).
- Interrupting an upload at 5 s left no file on the server.
- **No progress was shown during the 15 s upload.** See the confirmed bug [#88](https://github.com/jonbaldie/bookbeam-cli/issues/88).
- Uploading `Café — Ünïcode.epub` stored that exact filename (file 33). A default-name download wrote `Caf ncode.epub` with the correct bytes. The API's redirect already sets that ASCII-folded filename in its `Content-Disposition`, so this is server behaviour. Rejected as a CLI bug.
- Re-uploading `small.epub` created a second file with the same name (file 34). The API allows duplicate names.
- `links create 29 --json` carried `public_url`. That URL returned HTTP 200 and showed the project title and consent text. Answering `n` to link deletion and to project deletion left both in place. Forced project deletion succeeded. A follow-up GET returned 404, and the public link returned 404.

**Result:** ordinary transfers are correct. Interruption and progress fail. Two bugs filed.

### 3. Manage the team's subscriber webhook

**Goal:** an author sets a webhook, sees it in status, gets bad input rejected, and clears it only when they mean to. This is grounded in the README newsletter commands and the `--url` help (`empty string clears webhook`).

- `newsletter webhook --url https://example.com/bookbeam-exploratory-2026-10-24` was reflected in both the table and `--json`.
- `--url not-a-url` returned `422 The webhook url field must be a valid URL.` (with the usage block, #6). `--url ftp://example.com/x` was **accepted** by the API. That is server-side validation. Noted, not filed against the CLI.
- **`newsletter webhook` with no `--url` cleared the configured webhook without asking.** See the confirmed bug [#87](https://github.com/jonbaldie/bookbeam-cli/issues/87). `--url ""` also clears, which is documented.
- `newsletter configure` with no flags, and with `--provider` only, failed client-side and sent no request. The provider itself was not reconfigured.

**Result:** set and validate work. Omitting the flag is destructive. One bug filed.

## Confirmed bugs

### [#86](https://github.com/jonbaldie/bookbeam-cli/issues/86): Interrupted `files download` destroys the existing copy and leaves a truncated file

**Impact:** Ctrl-C during a re-download replaces a good local book with a truncated, unreadable file of the same name. A first download that is interrupted leaves a file that looks complete.

**Starting state:** a 26,215,629-byte EPUB on the server. `keep.epub` locally is a complete copy.

**Replay:** `bookbeam files download <project> <file> -o keep.epub`, then Ctrl-C after 1–2 s.

**Expected:** the previous `keep.epub` stays intact, and no partial file is left under the destination name.

**Actual:** the destination was 4,533,786 bytes and 2,297,370 bytes after SIGINT over an existing copy (**2/2**). Fresh destinations were left at 6,349,358 bytes (SIGINT) and 8,790,923 bytes (SIGTERM). `unzip -t` rejects them. Cause: `os.Create` truncates the file before `io.Copy` runs, and nothing cleans up afterwards.

### [#87](https://github.com/jonbaldie/bookbeam-cli/issues/87): `newsletter webhook` without `--url` silently clears the team webhook

**Impact:** running the bare command to inspect it, or by mistake, removes the team's signup integration. It exits 0, and with `--quiet` it prints nothing.

**Starting state:** a webhook set to `https://example.com/bookbeam-exploratory-2026-10-24`.

**Replay:** `bookbeam newsletter webhook`, then `bookbeam newsletter status --json`.

**Expected:** nothing changes without `--url` or an explicit clearing flag. The help names only an explicit empty string as the way to clear.

**Actual:** prints `✓ Cleared newsletter webhook URL.`, and status then shows `webhook_url: null`. **3/3**, once with `--quiet`.

### [#88](https://github.com/jonbaldie/bookbeam-cli/issues/88): `files upload` shows no progress despite the README

**Impact:** a large upload is silent for as long as it takes (15 s here for 25 MB). An author cannot tell whether it is progressing or hung.

**Replay:** `time bookbeam files upload <project> ./big.epub` with a file of about 25 MB.

**Expected:** progress output, per README "Upload a file with progress tracking" and the `--quiet` description "progress output".

**Actual:** one start line, nothing for about 15 s, then a success line. **2/2**: one complete upload and one interrupted at 5 s. The feature was never implemented; `git log -S progress` finds only the README line. Labelled `needs-triage` because the fix is a choice between implementing progress and changing the README.

## Other observations and candidate classification

- **Rejected as CLI bugs (server behaviour):** the ASCII-folded download filename for non-ASCII uploads, and the API accepting an `ftp://` webhook URL.
- **Known, not duplicated:** the usage block after API and validation errors, including under `--json` (#6, #44). Unauthenticated non-`whoami` commands show `401 Unauthenticated.` without the login hint (#76).
- **Usability, not filed:** API 404 messages expose Laravel internals, e.g. `No query results for model [App\Models\BookProject] 999999`.
- **Usability, not filed:** `files download` silently overwrites an existing file at its destination, whether named by `-o` or by the server's filename. Only the interruption case loses data unexpectedly (#86).
- **Usability, not filed:** the pagination footer is capitalised differently: `projects`/`downloaders` use `(Total: 7)`, `logs` uses `(total: 11)`.
- **Usability, not filed:** under `--json`, delete commands proceed without a prompt even without `--force`. This comes from reading the source (`Printer.Proceed`) and was not exercised. It is intentional per the code comment, but it is undocumented in the README.
- **Usability, not filed:** `newsletter configure --help` says `--endpoint` is "required for Mailcoach", but the CLI does not enforce it. Not exercised against the API, to avoid touching the provider.
- No unresolved candidates remain.

## Limitations and unexplored areas

- One production account. The team newsletter provider and the billing state were not changed. The webhook was changed only to an IANA `example.com` URL and restored to `null`.
- Interruption was by signal only. A server-side connection drop was not induced, though the code path is the same (`io.Copy` error with no cleanup).
- Browser device-flow login, `newsletter configure`/`disconnect` against the API, Windows, and shell completion sourcing were not exercised.
- Fetching the public signup page did not change the landing-page view metric (still 1527).

## Cleanup

- Project 29 was deleted along with files 31–34 and link 26. A GET returned 404 and the public link returned 404. The catalog is back to 7 projects / 12 files. The webhook is back to `null`. The provider is unchanged.
- The real config SHA-256 stayed `a488da2b…bac1`.
- The run's binary, module cache, isolated config, downloads, partial files, fixtures, and issue drafts were removed. See [`cleanup.log`](2026-10-24-evidence/cleanup.log).
- **Docker containers, images, and volumes created by this run: 0.** The ID sets were identical before and after. Pre-existing Docker resources were not removed.
