# Auth, credentials, and readiness coverage

Synced commit `14b3363649eb5f7889310bfc52979fee6c015176`.

| Inventory | Model action or real-code check | Source and atomicity |
|---|---|---|
| S1 | CheckDeadline, Sleep, RequestToken | `cmd/auth.go` `pollDeviceToken`. Guard, cancellable wait and HTTP call are separate transitions. `now` advances during sleep. Wait cancellation returns before requesting. HTTP context errors refine ResponseError. |
| S1 | Pending, SlowDown, UnauthorizedResponse, ResponseError | `cmd/auth.go` `pollBackoff`. Pending preserves interval, slow-down adds five, and 401 terminates even when its error type says pending or slow-down. Actual replay checks these cases. |
| S1 | TokenResponse | `cmd/auth.go` `parseDeviceToken`. Well-formed JSON can yield empty or nonempty tokens. Invalid JSON refines ResponseError. |
| S1 | CheckCancellation, StopCancelled, SaveToken, SaveFailure, Report | `cmd/auth.go` login checks context after polling, then saves, then reports success. Cancellation observed at that check prevents saving. Direct-token login bypasses polling and that context check. Browser start is a nonblocking callback. |
| S1 | TestFormalAuthCancellation, TestFormalAuthCancellationBeforeSave | `cmd/root.go` `waitContext` and `cmd/auth.go` login. Actual cancelled wait makes no request. A custom response body cancels on close after the token response, exercising the final context check before persistence. |
| S2 | Store, Commit | `pkg/config/config.go` `StoreToken` prepares a separate candidate. Memory commits only after save succeeds. Empty/new represent logout/login. |
| S2 | Init override/client | `pkg/config/config.go` `Token`, `cmd/root.go` `load`. Flags and environment override the stored token. Client is a snapshot taken at initialization. Replay checks eight concrete layer combinations. |
| S3 | Mkdir, Marshal, Create, Write, Close, Rename, Fail | `pkg/config/config.go` `save`, `createTemp`, `replace`. Private temporary creation precedes writing. Write and close must succeed before rename replaces the target. Failure before successful rename preserves the old target. Memory commits separately afterward. |
| S3 | TestFormalCredentialsFailedSaveIsAtomic, TestFormalCredentialsFailedSaveStages | Actual rename failure, injected creation failure, partial temporary write failure and close failure preserve old memory and old target. Creation/write/close cases also verify no temporary file remains. |
| S3 | TestFormalConfigLoadMissingAndInvalid | `pkg/config/config.go` `load`. Missing file yields defaults without creating a file; invalid JSON errors. |
| S4 | Resolve, ResolveFailure, Settings, Client, Printer, Authenticate, Run | `cmd/root.go` `load` and root pre-run, `cmd/authentication.go` `requireAuthentication`. Config failure prevents execution. A resolved invocation needs an effective token or a public command. Replay executes the real pre-run gate for all four combinations. |

## Bounds and assumptions

AuthPoll explores two polls, expiry two seconds and initial interval one second. The poll cap closes exploration; it is not a production timeout rule. Success, empty-token success, pending, slow-down, 401, other errors, save success/failure and cancellation are explored. The clock advances only at waits. The model checks safety rather than liveness or scheduling. Existing product tests cover initial interval normalization and device-code fetch/parse failure. TLS, browser behavior, server grant validity and wall-clock jumps are excluded.

Cancellation can be observed at a wait or at the post-poll context check. The model treats the wait result as one observation, and does not claim cancellation can prevent an HTTP request already in flight. Cancellation arriving after the final context check can race with the subsequent save; the save property requires that check to have succeeded, not that cancellation remains impossible afterward. Direct-token login has no new cancellation guarantee.

Credentials models one token mutation, three representative override states and no concurrent Settings callers. Marshal cannot fail for the current string-only Config. Partial temporary writes refine the failing Write transition, because they cannot affect the target file. Rename is an atomic successful replacement or a failure leaving the target unchanged, matching the supported filesystem boundary. Cleanup failure, concurrent readers/writers and durability after power loss are excluded. No failure is modeled after successful rename because the remaining memory assignment does not return an error. Permissions are checked under local Unix semantics.

Readiness models a fresh invocation with both configuration outcomes and the public/token gate. The public Boolean abstracts `requiresAuthentication`, including explicit public annotations and root help/completion exceptions. Command internals refine Run elsewhere. `wrapAPIErrors` changes error presentation, not admission to Run; that presentation is outside this state model. Reusing an initialized app after later failed load is excluded.

## Properties and fault sensitivity

| Baseline property | Failing model mutant | Real source fault caught by replay |
|---|---|---|
| BackoffOnlyOnSlowDown | AuthPoll 1 adds time on pending | pending-backoff |
| ReportRequiresSave | AuthPoll 2 reports after failed save | report-on-save-failure |
| WaitCancellationStopsPolling | AuthPoll 3 ignores cancelled wait | ignore-wait-cancellation |
| SaveRequiresCancellationCheck | AuthPoll 4 bypasses post-poll cancellation check | skip-pre-save-cancellation |
| UnauthorizedIsTerminal | AuthPoll 5 retries a 401 | retry-unauthorized |
| OverridesNeverPersisted | Credentials 1 writes invocation override | persist-override |
| PermissionsBeforeWrite | Credentials 2 creates nonprivate temporary | insecure-temporary |
| FailedSaveIsAtomic | Credentials 3 changes memory before save | early-memory-commit |
| CommandRequiresReady | Readiness 1 runs after failed resolution | run-after-resolution-failure |
| CommandRequiresAuthentication | Readiness 2 bypasses token/public gate | bypass-authentication |

Each mutant config must violate its named invariant. Findings retain Mutation 0 and test stronger desired properties, rather than weakening baseline properties.

- NoRequestAfterExpiry still fails after checking the deadline then sleeping past it. The real replay receives pending after a first 1.1-second wait and makes a successful second request after the two-second expiry.
- LoginHasToken still fails on `{}` response, saving an empty token and reporting login. Replay inspects actual JSON output and persisted config.
- LogoutClearsEffectiveToken still fails when an invocation override remains after clearing stored credentials. This is the documented layer boundary.
- ClientMatchesSettings still fails when StoreToken changes settings after an app's API client was constructed. Ordinary single-command execution exits before reuse matters.

FailedSaveIsAtomic was an old finding at `068bec0`. Current source fixes it by staging the candidate and replacing the file before committing memory. It is now a passing baseline with its own failing model and real source mutants.

## Running and evidence

`manifest.json` declares 17 TLC expectations: three passing baselines, ten failing mutants and four failing findings. `replay-map.json` maps additional Go tests into a disposable repository copy. Never install these tests or apply source mutations to the product checkout.

Run `mutate_real.py DISPOSABLE_CLONE LOGDIR` after installing the replay files. It requires each source pattern exactly once, catches all ten source faults through named replay failures, restores each file in finally, and reruns the full replay baseline. Compilation failure alone does not count as detection.

Maintenance first reproduced old drift in the disposable clone: polling/sleep replay compilation errors and the obsolete failed-save finding expecting changed memory. After synchronization, the complete Go replay suite passed and all ten source faults failed their named tests. The restored baseline passed. Auth logs are in the workspace audit directory `auth-check-results`; the top-level skill runner produces the reproducible combined evidence.
