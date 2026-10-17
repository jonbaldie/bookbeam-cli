# Exploratory Testing Report: bookbeam-cli

**Date:** 2026-10-17 (UTC)
**Build:** `main` at `068bec00159f1606d0281a998157eb7d50160f92`, Go 1.26.3, macOS arm64
**Interface:** the built `bookbeam` binary, using documented commands and flags
**Result:** three journeys exercised; one new confirmed bug filed as [#80](https://github.com/jonbaldie/bookbeam-cli/issues/80). No product changes made.

## Setup and evidence

Dependencies were downloaded with `GOFLAGS=-mod=readonly go mod download`, using `go.mod`/`go.sum`. Their SHA-256 checksums stayed unchanged. `GOMODCACHE` and the binary were inside `.exploratory-run/` in this workspace; the supplied `TMPDIR` and shared `GOCACHE` were retained. No dependency or build-output symlinks into another checkout were used.

Live commands targeted `https://bookbeam.app`, using the existing PAT through `BOOKBEAM_TOKEN` and an isolated `BOOKBEAM_CONFIG_DIR`. The real config was read, never edited. `whoami` established readiness: Jonathan Baldie, Jonathan's Team. The catalog started with seven projects (IDs 21, 6, 5, 4, 3, 2, 1). Metrics started at 8 downloads, 12 files, 7 projects, and 1515 views. Writes were confined to disposable project **28**, link **25**, and file **30**, then deleted. Auth variations used fake tokens and separate local configs.

Evidence in [`2026-10-17-evidence/`](2026-10-17-evidence/):

- [`setup.log`](2026-10-17-evidence/setup.log): build, revision, and unchanged dependency checksums.
- [`session.log`](2026-10-17-evidence/session.log): actions, stdout, stderr, and exit codes. PAT, provider secret, emails, and the checkout signature are redacted.
- [`bb.py`](2026-10-17-evidence/bb.py), [`journey1.py`](2026-10-17-evidence/journey1.py), and [`journey2.py`](2026-10-17-evidence/journey2.py): public-CLI drivers. **Re-running journey 2 creates and deletes one live project.**
- [`cleanup.log`](2026-10-17-evidence/cleanup.log): Docker inventory and workspace cleanup.

## Journeys exercised

### 1. Point the CLI at BookBeam and keep the saved token private

**Goal:** a headless user can reach the API when the host has a trailing slash or mixed case, reject an unusable host before any request, apply one-off overrides without saving them, and store a token so other users cannot read it. Grounded in the README host rule ("must start with `http://` or `https://`; a trailing slash is ignored"), the auth login/logout commands, and the owner-only config contract from #73.

- Trailing slash, `HTTPS://`, mixed-case hostname, `:443`, and a doubled slash all reached `whoami` successfully.
- Query, fragment, userinfo, a missing scheme, a missing hostname, a leading space, and an invalid percent-escape failed at load and named `--host` or `BOOKBEAM_HOST`. A schemeless `--host` on `auth login --token` wrote no config file.
- `http://127.0.0.1:9` and `https://[::1]` were accepted as hosts and failed as connection errors, as expected.
- Fresh login created a `0700` directory and `0600` file, returned JSON with `logged_in: true`, and left the config byte-identical after one-off `--host`/`--token` and environment overrides. Logout removed the token and kept the host.
- A pre-existing `0644` config was tightened to `0600` on login. The #73 fix holds. The directory already existed at `0755` and stayed `0755`; the file is no longer world-readable.
- Unauthenticated `whoami` exited 1 with the `bookbeam auth login` hint and no usage block.

**Result:** the new host contract and the permissions fix held on the paths above. Two variations are usability notes below, not new bugs.

### 2. Publish a book with a cover, a file, and a signup link

**Goal:** an author can attach a cover, replace or remove it without losing other fields, upload a book and download the same bytes, publish a signup link whose public page loads, and cancel a delete. Grounded in the README project, file, and link commands.

- Created project 28 with a title, description, and PNG cover. A separate fetch returned the same fields. The cover URL returned HTTP 200, `image/png`, 69 bytes, PNG magic.
- A title-only update kept the description and cover. An update with no flags printed success and changed nothing.
- `--cover` together with `--remove-cover`, and `--description` together with `--clear-description`, exited 1 and left the project unchanged.
- Replacing the cover changed the URL; the new URL was also a PNG. `--remove-cover` persisted a null cover and kept the title and description.
- A `.txt` upload was rejected and no file was stored. `two words.epub` uploaded as file 30 under that filename. Explicit and negotiated downloads matched the upload bytes (586). Answering `n` to file deletion left it in a fresh list.
- Created link 25. The table public URL returned HTTP 200. A trailing-slash `--host` still printed `https://bookbeam.app/download/k065WD3Ex2`. Title-only update kept the consent text; `--clear-consent` persisted null. Answering `n` left the link in a fresh list.
- Routed the disposable project to an existing mailing list with tags `sci-fi`, `readers`, `et`. `--clear-tags` cleared the tags and kept the list. The team newsletter provider was not reconfigured.
- Answering `n` to project deletion left it present. Forced deletion succeeded. A fresh GET returned 404, the public link returned 404, and the catalog IDs returned to the starting seven.

**Result:** the ordinary publish path and the cancel/conflict variations passed. `links list --json` still omits `public_url`; that remains [#8](https://github.com/jonbaldie/bookbeam-cli/issues/8).

### 3. Check billing, page through activity, and export subscribers

**Goal:** an author can see entitlement and the checkout link, page through reader activity, and export the subscribers shown in the downloader list. Grounded in the README billing, logs, and export commands.

- `billing status` showed paid access, no subscription, and the Founder's Lifetime Deal, in both the table and `--json`.
- `billing checkout --json` printed the checkout URL and did not need a browser. The human command printed the same URL and exited 0. A HEAD of that URL returned 302 to a Lemon Squeezy cart. No purchase was made.
- `logs --limit 3` showed page 1 of 4, total 11. Pages 2 and 4 showed the later rows. `--limit 101` asked the API for `per_page` 100. `--event signup` filtered the table locally; `--json` stayed unfiltered, as the flag help states.
- Project 5 listed 2 downloaders. `downloaders export 5` wrote a 179-byte CSV with those two rows. The explicit and default output files were byte-identical. `--json` reported `bytes_written: 179`. An empty project exported a 35-byte header-only CSV.
- `newsletter status` masked `api_token` as `********` in both the table path and `--json`. `whoami --json` masked the same field. `newsletter lists` returned 13 lists.

**Result:** billing, export, and redaction held. Paging past the end did not. That is the confirmed bug below.

## Confirmed bug: a page past the end says the collection does not exist

**Issue:** [#80 — Past-the-end list pages say the catalog, downloaders, or logs do not exist](https://github.com/jonbaldie/bookbeam-cli/issues/80).

**Impact:** an author who asks for the next page is told to create a project, that a project has no downloaders, or that no activity was recorded, even though the same command's `--json` output shows the real total.

**Starting state:** authenticated; 7 projects on one page; project 5 has 2 downloaders; 11 log events.

**Replay:**

```bash
bookbeam projects list --page 2
bookbeam downloaders list 5 --page 2
bookbeam logs --page 99 --limit 3
```

**Expected:** the page is past the last page, and the real total remains visible.

**Actual:** exit 0 and an empty-collection notice. `--json` shows `total` 7, 2, and 11 respectively. Repeated **2/2** for each command. `logs --event no-such-type` uses the same false logs notice (**2/2**); its `--json` output is unfiltered, which the flag help already states.

## Other observations and candidate classification

- **Known, not duplicated:** API errors and client-side validation errors still print a usage block, including under `--json`. Comment added to [#6](https://github.com/jonbaldie/bookbeam-cli/issues/6#issuecomment-6228230780). `links list --json` still omits `public_url`. Comment added to [#8](https://github.com/jonbaldie/bookbeam-cli/issues/8#issuecomment-6228230972).
- **Rejected as a CLI bug:** `projects newsletter --json` returned `files_count: 0` and `signup_links_count: 0` while a following `projects get` showed 1 and 1. The command prints the API document. The GET proved the file and link were still there.
- **Rejected as a product bug:** a host with a path, `--host https://bookbeam.app/api`, is a legal base URL under the current validator and then 404s at `api/api/v1/user`. The README requires a scheme and ignores a trailing slash; it does not reject a path.
- **Usability, not filed:** an invalid host stored in `config.json` blocks every command, including `auth logout`, so the token stays until the file is edited. The error names the file. The CLI itself does not save a schemeless host.
- **Usability, not filed:** `logs --limit 0`, `--limit -5`, and `--page 0` omit the query parameter and use the API default (`per_page` 15, page 1) instead of the flag default or an error. The page case is already in [#64](https://github.com/jonbaldie/bookbeam-cli/issues/64). The 2026-10-03 report noted the same silent fallback.
- **Usability, not filed:** `projects update` with no flags prints success and changes nothing. There is still no flag to clear a project's mailing-list id. `projects get` table does not show newsletter routing. These were already noted on 2026-10-03.
- **Usability, not filed:** deletion cancellation exits 0 and prints `Cancelled.` A later read is what shows the resource survived.
- No other unresolved failure candidates remain from the attempted journeys.

## Limitations and unexplored areas

- One production account. Live writes were limited to project 28. The team newsletter provider, webhook, and billing subscription were not changed. Checkout was opened only as far as the printed URL and a HEAD request.
- Browser device-flow login was not exercised. It needs a person to approve the code.
- No reader signup was submitted, so a populated export of a brand-new link was not created. The populated export used the existing project 5 subscribers.
- Fetching one public signup page increased landing-page views from 1515 to 1516. That is the only lasting catalog metric change.
- Large or interrupted transfers, Windows permissions, and `newsletter configure` / `disconnect` / `webhook` were not exercised.
- Shell completion `bash` emitted a script and exited 0. It was not sourced into a shell.

## Cleanup

- Link 25, file 30, and project 28 were deleted with the project. A fresh project GET returned 404. The public link returned 404. Catalog IDs and total returned to their starting values.
- Real config SHA-256 stayed `a488da2b03fe0582c61df018ca8e2e5d8bbb0729becf16ec812a02fc9428bac1`.
- Run-owned binary, module cache, fake-token configs, fixtures, downloads, and CSV exports were removed after this report was written. See [`cleanup.log`](2026-10-17-evidence/cleanup.log).
- **Docker containers, images, and volumes created by this run: 0.** Pre-existing Docker resources were not removed.
