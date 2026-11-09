---------------------------- MODULE Progress ----------------------------
EXTENDS Naturals
CONSTANT Mutant
VARIABLES written, queued, observed, returned
vars == <<written,queued,observed,returned>>
Init == /\ written = 0 /\ queued = 0 /\ observed = 0 /\ returned = FALSE
Write == /\ written < 2 /\ written' = written + 1
         /\ queued' = IF queued = 0 THEN (IF Mutant = 1 THEN written + 2 ELSE written + 1) ELSE queued
         /\ UNCHANGED <<observed,returned>>
Observe == /\ ~returned /\ queued > 0 /\ observed' = queued /\ queued' = 0
           /\ UNCHANGED <<written,returned>>
Return == /\ ~returned /\ returned' = TRUE /\ UNCHANGED <<written,queued,observed>>
Done == /\ returned /\ written = 2 /\ UNCHANGED vars
Next == Write \/ Observe \/ Return \/ Done
Spec == Init /\ [][Next]_vars
ProgressNeverOverstatesTransport == observed <= written
=============================================================================
