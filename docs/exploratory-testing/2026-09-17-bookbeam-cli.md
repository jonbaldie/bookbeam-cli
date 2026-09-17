# Exploratory Testing Report: bookbeam-cli

**Date:** 2026-09-17  
**Scope:** `bookbeam-cli` (v0.1.1 / current `main`)  
**Target Host:** `https://bookbeam.app` (Production)  
**Tester:** Agentic Exploratory Testing Pass (Pairing with maintainer)  

---

## 1. System Setup & Starting State

- **Binary:** Built directly from repo root (`go build -o bin/bookbeam .`, Go 1.24 on macOS darwin/arm64).
- **Configuration:** Ordinary user configuration at `~/.config/bookbeam/config.json`.
- **Authenticated Identity:** Verified via `bookbeam whoami`:
  - User: Jonathan Baldie (`jon@jonbaldie.com`)
  - Team: Jonathan's Team (ID: 1)
  - API Host: `https://bookbeam.app`
- **Initial Catalog State:** 6 existing book projects (IDs: 1, 2, 3, 4, 5, 6).
- **Verification Harness:** Public CLI commands exercised against live production backend with isolated transient resources, cleaned up after every test pass.

---

## 2. Journeys Exercised

Three core user-critical journeys were selected and thoroughly tested for both ordinary paths and variations:

### Journey 1: Book Project Lifecycle & Asset Management
- **Goal:** An author creates a new book project, configures title and metadata, uploads book files (EPUB/PDF), inspects project and asset details, downloads files, and cleans up assets and projects.
- **Ordinary Path:**
  - `bookbeam projects create --title "Exploratory Test Project" --description "Testing bookbeam-cli"` -> Successfully created project `#8`.
  - `bookbeam projects get 8` -> Accurately displayed metadata in table format.
  - `bookbeam projects update 8 --title "Exploratory Test Project Updated"` -> Updated title; verified lasting effect with `get`.
  - `bookbeam files upload 8 /tmp/test-book.pdf` -> Uploaded 316-byte PDF as file `#17`.
  - `bookbeam files list 8` -> Displayed uploaded file with size, type, and zero downloads.
  - `bookbeam files download 8 17 -o /tmp/downloaded.pdf` -> Downloaded file; `diff` confirmed byte-for-byte fidelity with original source.
  - `bookbeam files delete 8 17 --force` -> Successfully deleted file.
  - `bookbeam projects delete 8 --force` -> Successfully deleted project.
- **Variations Attempted:**
  - *Non-existent IDs:* `projects get 999999` and `files download 8 999999` returned clean API 404 errors (with Cobra usage blocks).
  - *Non-numeric IDs:* `projects get abc` returned HTTP 500 Server Error because string IDs bypass client validation and crash server query.
  - *Unsupported upload extensions:* `files upload 8 test.txt` was rejected client-side (`unsupported file extension '.txt'; allowed extensions are .epub, .mobi, .pdf`).
  - *0-byte file upload:* `files upload 12 empty.epub` was rejected by API with HTTP 422 (`The file field must be a file of type: epub, mobi, pdf.`).
  - *Confirmation prompts:* Tested answering `n` to interactive prompts for `projects delete` and `files delete`; verified cancellation and that resources remained intact.
  - *Cascade deletion:* Tested deleting a project (`#16`) containing active files and links; backend cleanly cascaded deletion and public links immediately returned HTTP 404.
- **Surprise & Confirmed Bug:**
  - Updating a project's cover image with `bookbeam projects update <id> --cover <path>` consistently crashes with HTTP 405 Method Not Allowed (**Issue #13**).

---

### Journey 2: Reader Acquisition & Signup Links Workflow
- **Goal:** An author creates reader lead-magnet landing page links, customizes title and newsletter opt-in consent copy, lists links with public delivery URLs, updates link copy, and deletes links.
- **Ordinary Path:**
  - `bookbeam links create <id> --title "Free Sample Chapters" --consent "I agree to receive author newsletters"` -> Successfully generated slug and public URL `https://bookbeam.app/download/7N8lx3zEE8`.
  - `bookbeam links list <id>` -> Rendered table with ID, Title, Slug, Public URL, and Creation Date.
  - `bookbeam links update <id> <link-id> --title "Updated Sample Chapters" --consent "New consent text"` -> Updated both attributes.
  - `bookbeam links delete <id> <link-id> --force` -> Successfully deleted link.
- **Variations Attempted:**
  - *Creating link without `--title`:* Handled gracefully by API (defaults title to "Untitled Link").
  - *Creating link with only `--consent`:* Handled gracefully by API.
  - *Cancellation prompts:* Interactive prompt cancelled cleanly on `n`.
  - *Updating only `--title`:* **Severe Data Loss Bug Discovered!** Updating only the title wiped out the link's existing `opt_in_text` to `null` (**Issue #14**).
  - *Updating only `--consent`:* **Crash Discovered!** Updating only consent copy failed with HTTP 422 `The title field is required.` (**Issue #14**).

---

### Journey 3: Analytics, Telemetry & Subscriber Exports
- **Goal:** An author checks aggregated platform engagement (`metrics`), reads reader activity logs (`logs`), reviews subscriber signups (`downloaders list`), and exports subscriber CSV records (`downloaders export`).
- **Ordinary Path:**
  - `bookbeam metrics` -> Displayed total projects (6), uploaded files (12), reader downloads (8), and page views (1455).
  - `bookbeam downloaders list 1` -> Displayed subscriber email, signup link name, and timestamp.
  - `bookbeam downloaders export 1 -o export.csv` -> Exported valid CSV file matching downloader table records.
- **Variations Attempted:**
  - *Empty downloader export:* Running `downloaders export` on project with 0 downloaders exported clean header-only CSV.
  - *Custom pagination:* `downloaders list --page 2` cleanly reported "No downloaders found for this project".
  - *JSON output on export:* Running `downloaders export <id> --json` wrote the CSV to disk but emitted 0 bytes to stdout because `--json` suppresses `PrintInfo` while no JSON output logic exists for CSV export.
  - *Log fetching:* Running `bookbeam logs` and `bookbeam logs -n 5` crashed decoding the paginator response structure (confirmed existing **Issue #1**).
  - *Billing commands:* Running `bookbeam billing status` and `bookbeam billing checkout` crashed decoding the active offer object (confirmed existing **Issue #2**).

---

## 3. Confirmed Bugs (Filed in GitHub Issues)

### Bug 1: `projects update --cover`: Fails with HTTP 405 Method Not Allowed
- **Issue:** [#13](https://github.com/jonbaldie/bookbeam-cli/issues/13)
- **User Impact:** Authors cannot update their book project cover images through the CLI. Any attempt fails 100% of the time.
- **Starting State:** Any existing book project.
- **Replay Steps:**
  ```bash
  bookbeam projects create --title "Cover Bug Demo"
  # Note returned project ID <id>
  bookbeam projects update <id> --cover ./cover.jpg
  ```
- **Observed Outcome:**
  ```
  Error: API error (405): The POST method is not supported for route api/v1/projects/<id>. Supported methods: GET, HEAD, PUT, PATCH, DELETE.
  ```
- **Root Cause:**
  In `cmd/projects.go` line 211, `projectsUpdateCmd` calls `apiCli.PostMultipart(fmt.Sprintf("/api/v1/projects/%s", projectID), fields, "cover_image", flagProjectCover)`.
  This executes an HTTP `POST` to `/api/v1/projects/<id>`.
  Because the BookBeam backend is a Laravel application, `/api/v1/projects/{id}` only accepts `PUT` and `PATCH`. PHP/Laravel requires method spoofing (`_method=PUT`) to handle multipart uploads for PUT routes.
  Direct verification via curl confirmed that passing `-F "_method=PUT"` to the endpoint succeeds immediately.

---

### Bug 2: `links update`: Crashes with 422 if `--title` omitted; silently deletes consent text if `--consent` omitted
- **Issue:** [#14](https://github.com/jonbaldie/bookbeam-cli/issues/14)
- **User Impact:**
  1. Authors attempting to update only newsletter consent text get a hard crash (`API error (422): The title field is required.`).
  2. Authors attempting to fix a typo in a link title silently delete their reader opt-in consent copy, causing legal / GDPR compliance issues on reader download pages.
- **Starting State:** A project with an existing signup link that has both a title and consent text.
- **Replay Steps:**
  ```bash
  # 1. Create a link with title and consent
  bookbeam links create <project-id> --title "Reader Magnet" --consent "Join my VIP mailing list"

  # 2. Inspect state
  bookbeam links list <project-id> --json
  # Note: "opt_in_text": "Join my VIP mailing list"

  # 3. Update only the title
  bookbeam links update <project-id> <link-id> --title "New Magnet Title"

  # 4. Re-check state
  bookbeam links list <project-id> --json
  # Result: "opt_in_text": null  <-- Consent text silently deleted!

  # 5. Attempt to update only consent text
  bookbeam links update <project-id> <link-id> --consent "Join my VIP mailing list"
  # Result: Error: API error (422): The title field is required.
  ```
- **Root Cause:**
  In `cmd/links.go` (`linksUpdateCmd`), the command constructs a partial map containing only the flags provided on the command line:
  ```go
  payload := map[string]string{}
  if flagLinkTitle != "" {
      payload["title"] = flagLinkTitle
  }
  if flagLinkOptIn != "" {
      payload["opt_in_text"] = flagLinkOptIn
  }
  raw, err := apiCli.Put(fmt.Sprintf("/api/v1/projects/%s/links/%s", projectID, linkID), payload)
  ```
  The BookBeam API endpoint `PUT /api/v1/projects/{project}/links/{link}` requires `title` on every PUT request, and sets `opt_in_text` to null if omitted from the PUT body. The CLI does not pre-fetch existing link data or merge flags.

---

## 4. Previously Logged Issues Verified During This Pass

| Issue | Title | Observed Status in Pass |
|---|---|---|
| [#1](https://github.com/jonbaldie/bookbeam-cli/issues/1) | logs: table view crashes decoding the API response | Confirmed (`json: cannot unmarshal object into Go value of type []cmd.ActivityLogItem`). Default `-n 50` also yielded `EOF`. |
| [#2](https://github.com/jonbaldie/bookbeam-cli/issues/2) | billing status: crashes, active_offer is an object not a string | Confirmed (`json: cannot unmarshal object into Go struct field BillingStatusResponse.active_offer of type string`). `billing checkout` also fails. |
| [#3](https://github.com/jonbaldie/bookbeam-cli/issues/3) | newsletter status/lists: report 'not configured' and 'no lists' | Confirmed (`Configured: No` despite working Mailcoach provider; `newsletter lists` shows empty table despite 13 lists present in JSON). |
| [#6](https://github.com/jonbaldie/bookbeam-cli/issues/6) | Suppress usage block after API errors (SilenceUsage) | Confirmed (Every 401, 404, 422, and 500 error prints full command usage block). |
| [#7](https://github.com/jonbaldie/bookbeam-cli/issues/7) | files download: saved as project-N-file-M with no extension | Confirmed (`files download` without `-o` drops original file extension). |
| [#8](https://github.com/jonbaldie/bookbeam-cli/issues/8) | links list --json omits public_url | Confirmed (`links list --json` omits `public_url`). |
| [#9](https://github.com/jonbaldie/bookbeam-cli/issues/9) | downloaders export: add -o / stdout option | Confirmed (`-o` exists, but `--json` produces empty stdout and writes file to disk). |

---

## 5. Usability & Ergonomic Observations

1. **Silent deletion vs explicit clearing:**
   - In `projects update`, passing `--description ""` does nothing because `flag != ""` check ignores empty strings.
   - In `links update`, passing `--title "..."` clears `opt_in_text` completely. There is currently no consistent convention for clearing optional string attributes.
2. **Missing Client-Side ID Validation:**
   - Passing non-integer IDs (e.g. `bookbeam projects get abc`) passes the raw string into URL paths and triggers unhandled HTTP 500 Internal Server Errors from the backend rather than friendly CLI validation errors like "project-id must be a numeric ID".
3. **Implicit Confirmation Bypass with `--json`:**
   - `projects delete`, `files delete`, and `links delete` all bypass interactive confirmation when `--json` is supplied (`if !flagForce && !printer.JSON`). While useful for scripting, users passing `--json` to inspect the response might not realize it implies `--force`.
4. **Unauthenticated Exit Code Inconsistency:**
   - `bookbeam whoami` when unauthenticated exits with status code 4 and prints a custom error message.
   - Other commands (`bookbeam projects list`, `bookbeam files list`) make an unauthenticated request, receive HTTP 401, dump the full usage block, and exit with status code 1.

---

## 6. Cleanup Verification

All temporary book projects created during this exploratory pass (IDs: `#8`, `#9`, `#10`, `#11`, `#12`, `#13`, `#14`, `#15`, `#16`) and all uploaded test files and download files were deleted. The live account's catalog remains in its exact starting state (6 original projects).
