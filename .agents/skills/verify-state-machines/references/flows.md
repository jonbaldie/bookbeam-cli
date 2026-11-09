# Checked user flows

These diagrams were generated with the statechart skill from the refreshed models at source commit `14b3363`. TLC 2.19 exhausts each finite graph. The diagrams combine states with the same user-visible meaning and hide terminal stuttering. They retain each state on the selected counterexample paths.

## Device login

NoRequestAfterExpiry asks that token requests start before expiry. The shortest counterexample has steps 0 through 7. A pending response leads back to the deadline check, the second wait crosses expiry, and the next request starts. A Go replay follows this sequence against a local HTTP server. Cancellation now stops waiting and is checked before saving.

```mermaid
---
title: "Device login and expiry"
---
stateDiagram-v2
  state "Legend" as legend {
    lg0: Breaks NoRequestAfterExpiry
    lg1: Path to the NoRequestAfterExpiry bug
  }
  class lg0 breaks
  class lg1 bugPath
  s0: Check deadline
  s1: Sleep
  s2: Request token
  s3: Cancellation requested
  s4: Cancelled
  s5: Read response
  s6: Check cancellation
  s7: Failed
  s8: Save token
  s9: Report success
  s10: Logged in
  s11: Poll sent after expiry
  s12: Timed out
  [*] --> s0
  s0 --> s1: code still valid
  s1 --> s2: wait before polling
  s1 --> s3: cancel command
  s2 --> s5: request token
  s3 --> s4: stop polling or saving
  s5 --> s6: server returns token
  s5 --> s0: authorization pending or server asks to slow down
  s5 --> s7: request or response fails or server rejects authorization
  s6 --> s3: cancel command
  s6 --> s8: command still active
  s8 --> s7: save fails
  s8 --> s9: save token
  s9 --> s10: report login
  s2 --> s11: request token
  s11 --> s6: server returns token
  s11 --> s0: authorization pending or server asks to slow down
  s11 --> s7: request or response fails or server rejects authorization
  s0 --> s12: deadline reached
  classDef breaks fill:#ffecee,stroke:#e72e3b,stroke-width:2.5px,color:#cd2834
  class s0 bugPath
  class s1 bugPath
  class s2 bugPath
  class s5 bugPath
  class s11 breaks
  class s11 bugPath
```

Regenerate with `/statechart` using `models/auth/AuthPoll.tla`, configuration `models/auth/AuthPoll.finding-expiry.cfg`, and `views/auth.mjs`. Choose a new output directory.

## Partial updates

PreserveConcurrentEdit asks that an omitted field keep a concurrent writer's value. The shortest counterexample has steps 0 through 4. Fetch the old value, allow another writer to change it, merge the old value, then PUT it. Link and newsletter replays send stale values. Data loss assumes a server accepting unversioned last-write-wins PUT.

```mermaid
---
title: "Partial updates can overwrite newer values"
---
stateDiagram-v2
  state "Legend" as legend {
    lg0: Breaks PreserveConcurrentEdit
    lg1: Path to the PreserveConcurrentEdit bug
  }
  class lg0 breaks
  class lg1 bugPath
  s0: Requested edits
  s1: Payload ready
  s2: Read failed
  s3: Existing values fetched
  s4: Update sent
  s5: Snapshot is stale
  s6: Stale payload ready
  s7: Newer value overwritten
  [*] --> s0
  s0 --> s1: all values supplied
  s0 --> s2: fetch missing values
  s0 --> s3: fetch missing values
  s1 --> s4: send full update
  s3 --> s1: fill missing values
  s3 --> s5: someone edits the preserved field
  s1 --> s6: someone edits the preserved field
  s5 --> s6: fill missing values
  s6 --> s7: send full update
  classDef breaks fill:#ffecee,stroke:#e72e3b,stroke-width:2.5px,color:#cd2834
  class s0 bugPath
  class s3 bugPath
  class s5 bugPath
  class s6 bugPath
  class s7 breaks
  class s7 bugPath
```

Regenerate with `/statechart` using `models/mutations/ReadMergeWrite.tla`, configuration `models/mutations/ReadMergeWrite.finding.cfg`, and `views/merge.mjs`. Choose a new output directory.

## Deletion authorization

AuthorizedDelete holds. Invalid IDs prevent DELETE. Valid IDs require force, JSON mode, or an affirmative answer. Refusal and EOF cancel. JSON mode authorizes deletion under the implemented policy.

```mermaid
---
title: "Deletion authorization"
---
stateDiagram-v2
  s0: Validate IDs
  s1: Decide whether to proceed
  s2: No delete sent
  s3: Deletion authorized
  s4: Delete sent
  [*] --> s0
  s0 --> s2: invalid IDs
  s0 --> s1: IDs accepted
  s1 --> s2: refuse or end input
  s1 --> s3: answer yes or force flag or JSON script mode
  s3 --> s4: send DELETE
```

Regenerate with `/statechart` using `models/mutations/DeleteGate.tla`, configuration `models/mutations/DeleteGate.cfg`, and `views/delete.mjs`. Choose a new output directory.

## Download publication

FailurePreservesDestination and PublicationAfterClose hold. Book downloads and CSV exports write temporary files and rename only after successful transfer and close. CSV exports also check cancellation before rename. Existing files survive failure. Stdout streams bytes directly, reports short writes as errors, and cannot roll back emitted bytes.

```mermaid
---
title: "Downloads publish only completed files"
---
stateDiagram-v2
  s0: Existing destination
  s1: Write temporary file
  s2: Stream CSV to stdout
  s3: Transfer finished
  s4: Close temporary file
  s5: Check stream result
  s6: Error, previous file preserved
  s7: Stream error
  s8: Complete file published
  s9: CSV streamed
  [*] --> s0
  s0 --> s1: download book or export CSV to file
  s0 --> s2: export CSV to stdout
  s1 --> s3: transfer bytes
  s2 --> s3: transfer bytes
  s3 --> s4: close file and check cancellation
  s3 --> s5: finish stream
  s4 --> s6: return error
  s4 --> s8: rename completed file
  s5 --> s9: report stream success
  s5 --> s7: return error
```

Regenerate with `/statechart` using `models/io/Download.tla`, configuration `models/io/Download.cfg`, and `views/download.mjs`. Choose a new output directory.

## Scope

Login starts after the device code is decoded. Updates abstract HTTP errors and field values. Deletion ends at dispatch. Downloads abstract byte contents and filesystem outcomes. The charts do not prove filesystem durability, production server persistence, dependency internals, or liveness under arbitrary scheduling. Read [coverage.md](coverage.md) and the source maps for full bounds.
