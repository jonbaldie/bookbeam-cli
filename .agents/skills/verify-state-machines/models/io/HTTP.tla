------------------------------ MODULE HTTP ------------------------------
EXTENDS Naturals
CONSTANT Mutant
VARIABLES phase, mode, status, owner, deadline, cancelled, dispatched
vars == <<phase, mode, status, owner, deadline, cancelled, dispatched>>
Init == /\ phase = "choose" /\ mode = "none" /\ status = 0 /\ owner = "none"
        /\ deadline = "none" /\ cancelled = FALSE /\ dispatched = FALSE
Choose == /\ phase = "choose" /\ mode' \in {"short", "raw", "multipart", "transfer"}
          /\ cancelled' \in BOOLEAN /\ phase' = "dispatch"
          /\ UNCHANGED <<status, owner, deadline, dispatched>>
Cancel == /\ phase = "dispatch" /\ cancelled /\ Mutant # 4 /\ phase' = "done"
          /\ UNCHANGED <<mode,status,owner,deadline,cancelled,dispatched>>
Dispatch == /\ phase = "dispatch" /\ (~cancelled \/ Mutant = 4)
            /\ deadline' = IF mode = "short" /\ Mutant # 2 THEN "short"
                            ELSE IF mode = "transfer" /\ Mutant # 3 THEN "24h" ELSE "none"
            /\ dispatched' = TRUE /\ status' \in {200,399,400,500} /\ phase' = "response"
            /\ UNCHANGED <<mode,owner,cancelled>>
Classify == /\ phase = "response" /\ phase' = "owned"
            /\ owner' = IF status >= 400 THEN (IF Mutant = 1 THEN "caller" ELSE "client") ELSE "caller"
            /\ UNCHANGED <<mode,status,deadline,cancelled,dispatched>>
Finish == /\ phase = "owned" /\ phase' = "done" /\ owner' = "closed"
          /\ UNCHANGED <<mode,status,deadline,cancelled,dispatched>>
Done == /\ phase = "done" /\ UNCHANGED vars
Next == Choose \/ Cancel \/ Dispatch \/ Classify \/ Finish \/ Done
Spec == Init /\ [][Next]_vars
ErrorOwnership == phase = "owned" /\ status >= 400 => owner = "client"
ShortDeadline == dispatched /\ mode = "short" => deadline = "short"
TransferDeadline == dispatched /\ mode = "transfer" => deadline = "24h"
CancelledBeforeDispatch == cancelled => ~dispatched
AllRequestsHaveDeadline == dispatched => deadline # "none"
=============================================================================
