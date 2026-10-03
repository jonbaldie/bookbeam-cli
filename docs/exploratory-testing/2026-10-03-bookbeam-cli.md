# Exploratory Testing Report: bookbeam-cli

**Date:** 2026-10-03
**Scope:** `bookbeam-cli` at `main` `8824049` (after the #42, #47, #49, #52, #53 and #54 fixes)
**Target host:** `https://bookbeam.app` (production)
**Tester:** unattended agentic exploratory pass
**Evidence:** [`2026-10-03-evidence/`](2026-10-03-evidence/). Contents:
- [`session.log`](2026-10-03-evidence/session.log): every command, its stdout, stderr and exit code
- [`json-replay.txt`](2026-10-03-evidence/json-replay.txt) and [`secret-replay.txt`](2026-10-03-evidence/secret-replay.txt): bug replays
- [`r.sh`](2026-10-03-evidence/r.sh): the driver

The evidence has emails, the PAT, the provider secret and storage keys redacted.

---

## 1. Setup and starting state

- **Binary:** `go build -o /tmp/bbx/bookbeam .` (Go 1.26.3, darwin/arm64).
- **Configuration:** the ordinary user config `~/.config/bookbeam/config.json`, with the host set to `https://bookbeam.app` and a PAT. Auth tests ran in scratch `BOOKBEAM_CONFIG_DIR` directories. The real config's SHA-1 was `4d9339bb…` before the pass and unchanged after it.
- **Identity:** `whoami` showed Jonathan Baldie, Jonathan's Team, host `https://bookbeam.app`.
- **Starting catalog:** 7 projects (1–6 and 21). Metrics showed 12 files, 8 downloads and 1483 views. Project 21 ("Exploratory Journey 2 Links") is left over from an earlier run and was not touched.
- **Newsletter:** a Mailcoach provider is configured.

## 2. Journeys exercised

### Journey 1: Project lifecycle and newsletter routing

**Goal:** an author creates a project with a cover, edits its metadata, routes it to a mailing list with tags, and deletes it. Each change should show up in `projects get --json`.

- **Ordinary path:** `projects create --title … --description … --cover cover.png` created #26 with a cover URL. I ran each of these, then checked the result with `get --json`:
  - `update --title`
  - `update --description`
  - `update --cover`
  - `update --title … --cover …`
  - `update --clear-description`
  - `update --remove-cover`

  In every case the changed field updated and the other fields were preserved.
- **Newsletter routing:**
  - `projects newsletter 26 --list-id <uuid> --tags "sci-fi, readers ,et"` stored `["sci-fi","readers","et"]`, trimmed.
  - `--tags` alone kept the list.
  - `--list-id` alone kept the tags.
  - `--clear-tags` cleared the tags.
- **Variations:**
  - `--clear-description --description x` and `--cover … --remove-cover` were rejected client-side, and the state was unchanged.
  - `--clear-description --cover …` cleared the description and replaced the cover through the multipart path.
  - Malformed IDs were all rejected with `invalid project id "…"`, and no request was sent: `26abc`, `-5`, `0`, `99999999999999999999`, `" 26"` and `026`.
  - `projects get 99999` → `API error (404)`.
- **Result:** no CLI defects found. The fixes for #13, #23, #36, #47, #49 and #52 held.

### Journey 2: Book files and reader signup links

**Goal:** an author uploads book files, downloads them back intact, and publishes a signup link whose copy they can edit without losing consent text.

- **Files:**
  - I uploaded `book.epub` twice, plus `Mÿ Böök (final).EPUB` and `two words.epub`.
  - `files download` without `-o` saved each file under its original name. `cmp` showed a byte-identical round trip.
  - `files download --json` → `{"project_id","file_id","path","bytes_written"}` (#53 fix holds).
- **Links:**
  - `links create --title --consent` gave a public URL that returned HTTP 200.
  - `--title` alone kept the consent text.
  - `--consent` alone kept the title.
  - `--clear-consent` set `opt_in_text` to null.
  - `--consent` together with `--clear-consent` was rejected.
  - An unknown link and a cross-project link (`links update 1 21`) were rejected client-side with `signup link #N not found in project P`.
- **Deletes:** answering `n` at the prompts for `links delete`, `files delete` and `projects delete` cancelled them, and the resources survived. `--force` deleted. Deleting project #26 cascaded, and its public link then returned 404.
- **Result:** no CLI defects found. The fixes for #7, #12 and #14 held.

### Journey 3: Telemetry, subscriber export, and auth/config

**Goal:** an author pages through activity logs, exports subscribers to CSV, and manages login state without one-off overrides leaking into saved config.

- **Logs:**
  - `logs --limit 3` → "Page 1 of 4 (total: 11)".
  - `--page 2` and `--page 4` showed the correct pages.
  - `--event signup` filtered locally.
  - `--page 99` → "No activity logs recorded."
  - The #54 fix holds.
- **Downloaders:**
  - `downloaders list 5` showed 2 rows.
  - `downloaders export 5` and `-o sub.csv` wrote a 179-byte CSV that matched the list.
  - An empty project exported a header-only CSV.
  - `downloaders export --json` printed **nothing** to stdout (confirmed bug, #58).
- **Auth and config** (in a scratch config dir):
  - With no token, `whoami` gave a friendly hint and exited 1.
  - `auth login --token` saved `config.json` with mode 0600 in a 0700 directory.
  - `BOOKBEAM_TOKEN`, `--token`, `--host` and `BOOKBEAM_HOST` overrides worked for single runs and left `config.json` byte-identical.
  - `--host X auth login --token` saved the token but not the host (the intended behaviour from #42).
  - `auth logout` removed the token and kept the host.
  - `auth login --json` and `auth logout --json` printed **nothing** to stdout (#58).
- **Surprise:** `whoami --json` printed the team's Mailcoach API secret in plaintext. `newsletter status --json` masks the same field as `********` (confirmed bug, #59).

## 3. Confirmed bugs

### #58: `downloaders export`, `auth login --token` and `auth logout` print 0 bytes under `--json`

- **Issue:** https://github.com/jonbaldie/bookbeam-cli/issues/58
- **Impact:** the README says every command supports `--json`. Scripts that pipe these commands into `jq` get an empty stream and can't read the result path or status. This is the same defect #53 fixed for `files download`.
- **Starting state:** authenticated, a project with downloaders, and a scratch `BOOKBEAM_CONFIG_DIR` for the auth steps.
- **Replay:**
  ```bash
  bookbeam downloaders export 5 -o r.csv --json | wc -c   # 0 (r.csv written, 179 bytes)
  bookbeam auth login --token "$T" --json | wc -c         # 0
  bookbeam auth logout --json | wc -c                     # 0
  bookbeam files download 26 25 -o c.epub --json | wc -c  # 85 (control)
  ```
- **Expected:** a JSON document describing the result, as `files download --json` prints.
- **Actual:** empty stdout, exit 0.
- **Repeats:** 2 of 2 runs ([`json-replay.txt`](2026-10-03-evidence/json-replay.txt)). The 2026-09-17 pass also saw the export case.
- **Cause:** these handlers call `printer.Info` and then `return nil`:
  - `cmd/downloaders.go:111`
  - `cmd/auth.go:52`
  - `cmd/auth.go:105`
  - `cmd/auth.go:89`, the device flow, which I identified from the source only.

### #59: `whoami --json` prints the newsletter provider API secret in plaintext

- **Issue:** https://github.com/jonbaldie/bookbeam-cli/issues/59
- **Impact:** `whoami` output is what users paste when debugging auth. It contains `current_team.newsletter_provider_config.api_token`, which gives access to the author's subscriber lists.
- **Starting state:** authenticated, with a provider configured on the team.
- **Replay:**
  ```bash
  bookbeam newsletter status --json | jq .config.api_token                       # "********"
  bookbeam whoami --json | jq .current_team.newsletter_provider_config.api_token # full secret
  ```
- **Expected:** the secret is masked or left out, as `newsletter status` handles it.
- **Actual:** the full secret is printed.
- **Repeats:** 2 of 2 runs, across 6 commands. Only `whoami --json` and `auth whoami --json` leak ([`secret-replay.txt`](2026-10-03-evidence/secret-replay.txt)).
- **Cause:** the root cause appears to be in `GET /api/v1/user` on the server. The CLI passes that document through (`cmd/auth.go` `whoamiCmd`).

## 4. Rejected and unresolved candidates

- **Rejected: non-ASCII upload names were mangled on download.** `Mÿ Böök (final).EPUB` came back as `M Bk final.EPUB`. The server's S3 presigned URL already carries `filename="M Bk final.EPUB"`, so the CLI's `sanitizeFilename` is not the cause. This is server-side.
- **Rejected: `--host X auth login --token` didn't persist the host.** This is the intended behaviour from #42.
- **Unresolved, not exercised: the device-flow `auth login`.** It needs a browser. Its `--json` behaviour is inferred from the source (see #58).

## 5. Usability observations

These are observations, not filed bugs.

1. **Commands with no effect still report success:**
   - `projects update <id>` and `links update <p> <l>` with no flags print `✓ Updated …` and exit 0.
   - `links update --title ""` does the same.
   - `projects newsletter --list-id ""` fails with "specify --list-id, --tags, or --clear-tags", even though `--list-id` was given.
2. **There is no way to unroute a project from a mailing list.** `projects newsletter` has `--clear-tags` but nothing that clears the list.
3. **`projects newsletter --list-id bogus-list` is accepted and saved.** The API doesn't validate the list ID against the provider's lists.
4. **`projects get` doesn't show newsletter routing in its table.** Only `--json` shows the list ID and tags.
5. **`files download` silently overwrites an existing file.** Downloading two files that are both named `book.epub` left one file. `-o -` creates a file literally named `-` (related to #9).
6. **`auth login --token <anything>` saves the token without validating it.** The user only learns it's wrong on the next command.
7. **Out-of-range input isn't flagged:**
   - `logs --limit 0`, `--limit -5` and `--page 0` fall back to defaults silently.
   - `logs --page 99` says "No activity logs recorded" rather than reporting that the page is past the last one.
8. **Already-tracked issues were still visible during this pass:**
   - The API error usage block (#6).
   - `links list --json` without `public_url` (#8).
   - `--json` implying delete consent (#50).

## 6. Limitations

- This pass ran against production, and only one account and team were available.
- Device-flow login, billing checkout, and `newsletter configure`/`webhook`/`disconnect` were not exercised. Changing the live provider configuration risks real subscriber delivery.
- Large or slow transfers (#12) were not re-tested. The fixtures were under 1 KB.

## 7. Cleanup

- Project #26 was deleted. One file and one link had already been deleted individually; the cascade removed the remaining 3 files and 2 links.
- The catalog is back to 7 projects, and metrics match the start.
- The real `config.json` is unchanged (SHA-1 matches).
- Scratch config dirs and `/tmp/bbx` fixtures were removed.
