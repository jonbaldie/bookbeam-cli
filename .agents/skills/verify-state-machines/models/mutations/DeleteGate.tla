---------------------------- MODULE DeleteGate ----------------------------
EXTENDS TLC
CONSTANT Mutant
VARIABLES resource, phase, valid, force, json, affirmative, authorized, deleted
vars == <<resource, phase, valid, force, json, affirmative, authorized, deleted>>
Init == /\ resource \in {"project", "file", "link"} /\ phase = "validate"
        /\ valid \in BOOLEAN /\ force \in BOOLEAN /\ json \in BOOLEAN /\ affirmative \in BOOLEAN
        /\ authorized = FALSE /\ deleted = FALSE
Validate == /\ phase = "validate" /\ phase' = IF valid THEN "authorize" ELSE "done"
            /\ UNCHANGED <<resource, valid, force, json, affirmative, authorized, deleted>>
Authorize == /\ phase = "authorize" /\ authorized' = (force \/ json \/ affirmative)
             /\ phase' = IF (force \/ json \/ affirmative \/ Mutant) THEN "delete" ELSE "done"
             /\ UNCHANGED <<resource, valid, force, json, affirmative, deleted>>
Delete == /\ phase = "delete" /\ deleted' = TRUE /\ phase' = "done"
          /\ UNCHANGED <<resource, valid, force, json, affirmative, authorized>>
Done == /\ phase = "done" /\ UNCHANGED vars
Next == Validate \/ Authorize \/ Delete \/ Done
AuthorizedDelete == deleted => (valid /\ authorized)
Spec == Init /\ [][Next]_vars
=============================================================================
