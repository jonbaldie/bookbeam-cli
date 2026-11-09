---
name: verify-state-machines
description: Verify BookBeam CLI state transitions after changes to authentication, configuration, partial updates, deletion, HTTP resources, uploads, downloads, output, or release workflows.
---

# Verify BookBeam state machines

Run from the repository root.

```sh
python3 .agents/skills/verify-state-machines/scripts/verify.py
```

The command verifies source hashes, checks bounded TLA+ models with TLC 2.19 from checksum-pinned tla2tools release 1.7.4, and runs Go replay tests in a temporary repository copy. It also injects authentication, configuration, and file-transfer faults and removes JSON deletion authorization in that copy. Each fault must cause a replay failure. Restored source must pass. It leaves the working source untouched. Python 3, Go 1.26.3 or compatible, and Java 11 or later are required. The first run downloads TLC from its official GitHub release.

Read every TLA+ result with `/statechart`. Use its installed skill instructions to generate the diagrams. Replay each new counterexample through the actual Go functions before calling it a code finding.

Read [findings.md](references/findings.md) for the existing counterexamples and their code replays. Read [flows.md](references/flows.md) for the checked user-flow diagrams. The corresponding view modules are in `views/`.

Read [coverage.md](references/coverage.md) when a covered function changes. Read the relevant `models/*/source-map.md` for transition boundaries and assumptions. The source snapshot was reviewed at commit `14b3363649eb5f7889310bfc52979fee6c015176`. [source.json](references/source.json) records the exact files and hashes.

If the source guard fails, inspect the changed code against the relevant transitions and replay tests. Update the model when behavior changes. Move the recorded commit and hashes forward only after confirming that the verification describes the new source. Use `/maintain-formal-verification-skill` for changes that accumulated without this review.

Each model manifest distinguishes baseline checks, deliberate model mutants, and existing findings. A baseline must pass. A mutant must violate its named property. An existing finding must retain its named counterexample until an intended product fix changes that behavior. A missing or different result fails the runner. Preserve findings when reviewing unrelated changes.

This verifies finite abstractions and representative code correspondence. It does not prove the production server, Go runtime, dependency internals, filesystem durability, unbounded time, or all network behavior. Model mutants demonstrate property sensitivity. The separate real source mutation demonstrates that the Go replay suite can catch a production-code regression.
