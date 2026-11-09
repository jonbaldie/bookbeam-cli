-------------------------- MODULE ReadMergeWrite --------------------------
EXTENDS TLC
CONSTANT Mutant
VARIABLES flow, phase, missing, readOK, snapshot, remote, payload, changed, sent
vars == <<flow, phase, missing, readOK, snapshot, remote, payload, changed, sent>>
Init == /\ flow \in {"link", "newsletter"} /\ phase = "edits"
        /\ missing \in BOOLEAN /\ readOK \in BOOLEAN
        /\ snapshot = "none" /\ remote = "old" /\ payload = "none"
        /\ changed = FALSE /\ sent = FALSE
ReadExisting == /\ phase = "edits" /\ missing
                /\ phase' = IF readOK THEN "snapshot" ELSE "error"
                /\ snapshot' = IF readOK THEN remote ELSE "none"
                /\ UNCHANGED <<flow, missing, readOK, remote, payload, changed, sent>>
FullEdit == /\ phase = "edits" /\ ~missing
            /\ phase' = "ready" /\ payload' = "requested"
            /\ UNCHANGED <<flow, missing, readOK, snapshot, remote, changed, sent>>
Merge == /\ phase = "snapshot" /\ phase' = "ready" /\ payload' = snapshot
         /\ UNCHANGED <<flow, missing, readOK, snapshot, remote, changed, sent>>
ConcurrentEdit == /\ phase \in {"snapshot", "ready"} /\ missing /\ ~changed
                  /\ remote' = "newer" /\ changed' = TRUE
                  /\ UNCHANGED <<flow, phase, missing, readOK, snapshot, payload, sent>>
Put == /\ (phase = "ready" \/ (Mutant /\ phase = "error"))
       /\ phase' = "done" /\ sent' = TRUE /\ remote' = payload
       /\ UNCHANGED <<flow, missing, readOK, snapshot, payload, changed>>
Done == /\ phase \in {"done", "error"} /\ UNCHANGED vars
Next == ReadExisting \/ FullEdit \/ Merge \/ ConcurrentEdit \/ Put \/ Done
ReadFailurePreventsWrite == sent => (~missing \/ readOK)
PreserveConcurrentEdit == (phase = "done" /\ missing /\ changed) => remote = "newer"
Spec == Init /\ [][Next]_vars
=============================================================================
