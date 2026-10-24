# Documentation

Documentation index for `bookbeam-cli`.

## Exploratory Testing

- [2026-09-17 Exploratory Testing Pass](exploratory-testing/2026-09-17-bookbeam-cli.md) — Live pass against `https://bookbeam.app` covering project lifecycles, file asset management, reader signup links, telemetry, and confirmed issues (#13, #14).
- [2026-10-03 Exploratory Testing Pass](exploratory-testing/2026-10-03-bookbeam-cli.md) — Live pass against `https://bookbeam.app` covering project lifecycle and newsletter routing, files and signup links, telemetry/export and auth/config overrides; confirmed issues (#58, #59).

- [2026-10-10 Exploratory Testing Pass](exploratory-testing/2026-10-10-bookbeam-cli.md) — Auth/config persistence, project and signup-link edits, small-file round trips and empty CSV export; confirmed existing-config permissions bug (#73).
- [2026-10-17 Exploratory Testing Pass](exploratory-testing/2026-10-17-bookbeam-cli.md) — Host canonicalisation and credential storage, cover/file/signup-link publishing, billing, telemetry, and subscriber export; confirmed past-the-end list pages report an empty collection (#80).
- [2026-10-24 Exploratory Testing Pass](exploratory-testing/2026-10-24-bookbeam-cli.md) — `--json` derived fields and past-the-end paging, full-size file transfers and interruption, and team webhook management; confirmed interrupted downloads truncate existing copies (#86), bare `newsletter webhook` clears the webhook (#87), and missing upload progress (#88).

## Agent Guidelines

- [Domain Docs](agents/domain.md)
- [Issue Tracker Operations](agents/issue-tracker.md)
- [Triage Labels](agents/triage-labels.md)
