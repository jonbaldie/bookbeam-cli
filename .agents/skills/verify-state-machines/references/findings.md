# Counterexamples and policy boundaries

These stronger properties fail on the reviewed source. The replay tests pass by confirming current behavior. They do not establish that the desired properties hold. Product behavior has not been changed.

| Property | Shortest model path and consequence | Code evidence and classification |
|---|---|---|
| NoRequestAfterExpiry | Steps 0-7. Check deadline, wait, poll, receive pending, check deadline, wait past expiry, poll again. | `TestFormalAuthFindingPollAfterExpiry` follows the sequence with an actual HTTP server. A request can begin after expiry. |
| LoginHasToken | Steps 0-7. Poll, accept an empty decoded token, save it, report login. | `TestFormalAuthFindingEmptyTokenLogin` executes the login handler with a successful `{}` token response. The JSON result says logged in while the saved token is empty. |
| LogoutClearsEffectiveToken | Steps 0-8. Start with an invocation override, clear the stored token, save. The override remains effective. | `TestFormalCredentialsFindingLogoutOverride`. Documented precedence policy, rather than a claim of incorrect logout implementation. |
| ClientMatchesSettings | Steps 0-8. Initialize a client, replace the stored token, save. The client keeps its original snapshot. | `TestFormalAuthReadinessAndClientSnapshot`. Relevant when reusing the app in-process. Ordinary one-command execution exits. |
| PreserveConcurrentEdit | Steps 0-4. Read a missing field, another writer edits it, merge the snapshot, PUT the old value. | `TestFormalStaleLinkOverwriteReplay` and `TestFormalStaleNewsletterOverwriteReplay` use actual handlers and observe stale request values. Overwrite assumes server acceptance of unversioned last-write-wins PUT. |
| ErrorMeansNoMutationRequest | Steps 0-3. Validate, send a create request, fail decoding the response, return an error. | `TestFormalRemoteCreateThenDecodeFailureReplay` records the request before returning malformed JSON. An error is insufficient evidence that the remote action did not happen. Production server persistence is not proven. |
| AllRequestsHaveDeadline | Steps 0-2. Choose a raw request or multipart upload and dispatch without a fixed deadline. | `TestFormalHTTPDeadlineAndOwnership` inspects real request contexts. Raw Request and multipart calls depend on the supplied context. StreamGet now installs the transfer deadline, defaulting to 24 hours. A short-call timeout still does not apply to every request. |
| ReturnMeansFileClosed | Steps 0-2. Start multipart upload, request fails, caller closes the reader and returns before producer cleanup runs. | Model scheduling observation only. `TestFormalMultipartFailureEventuallyClosesFile` confirms eventual cleanup but does not force this exact schedule. This is not a confirmed leak or a concretely replayed bug. |

The selected [flow diagrams](flows.md) retain the counterexample states and marks. Each full statechart report also checks deadlocks. The broken multipart reader-cleanup mutant introduces a deadlock as well as its intended ownership violation. That deadlock exists only in the deliberately broken variant.

The source maps explain abstraction boundaries. Reproduce new counterexamples with the actual functions before calling them code findings. Keep server assumptions attached to any claim about remote data.

## Fixed in the refreshed source

The original snapshot at `068bec0` had two additional counterexamples. They are passing baseline properties at `14b3363`, with separate broken-model sensitivity checks.

- `FailedSaveIsAtomic` now holds. Credentials are saved through a private temporary file and rename before in-memory state commits. Failure replays preserve the old memory and disk values. This does not prove durability after a machine crash.
- `FailurePreservesDestination` now holds for book downloads and CSV file exports. Transfers write to temporary files, close them, then rename to publish. Failure replays retain existing destination bytes. CSV stdout is a stream and cannot roll back bytes already written.

The landing maintenance inspected all intervening production and workflow changes before refreshing the source guard. The remaining findings above are retained rather than waived.
