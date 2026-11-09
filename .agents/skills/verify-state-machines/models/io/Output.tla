----------------------------- MODULE Output -----------------------------
EXTENDS Naturals
CONSTANT Mutant
VARIABLES phase, mode, json, quiet, prepared, published, report, opened, footer
vars == <<phase,mode,json,quiet,prepared,published,report,opened,footer>>
Init == /\ phase = "choose" /\ mode = "none" /\ json = FALSE /\ quiet = FALSE
        /\ prepared = FALSE /\ published = FALSE /\ report = "none" /\ opened = FALSE /\ footer = FALSE
Choose == /\ phase = "choose" /\ phase' = "prepare" /\ mode' \in {"launch","display"}
          /\ json' \in BOOLEAN /\ quiet' \in BOOLEAN
          /\ UNCHANGED <<prepared,published,report,opened,footer>>
PrepareOK == /\ phase = "prepare" /\ prepared' = TRUE /\ phase' = "publish"
             /\ UNCHANGED <<mode,json,quiet,published,report,opened,footer>>
PrepareFails == /\ phase = "prepare" /\ json /\ phase' = "finish" /\ report' = "error"
                /\ published' = (Mutant = 1)
                /\ UNCHANGED <<mode,json,quiet,prepared,opened,footer>>
Publish == /\ phase = "publish" /\ phase' = "finish" /\ published' = TRUE
           /\ report' \in (IF json \/ mode = "display" THEN {"ok","error"} ELSE {"ok"}) /\ UNCHANGED <<mode,json,quiet,prepared,opened,footer>>
Finish == /\ phase = "finish" /\ phase' = "done"
          /\ opened' = (mode = "launch" /\ ((report = "ok" /\ ~json) \/ Mutant = 2))
          /\ footer' = (mode = "display" /\ ~json /\ (report = "ok" \/ Mutant = 3))
          /\ UNCHANGED <<mode,json,quiet,prepared,published,report>>
Done == /\ phase = "done" /\ UNCHANGED vars
Next == Choose \/ PrepareOK \/ PrepareFails \/ Publish \/ Finish \/ Done
Spec == Init /\ [][Next]_vars
PreparationBeforePublication == published => prepared
LaunchAfterSuccessfulText == opened => ~json /\ report = "ok"
NoFooterAfterError == footer => report = "ok"
=============================================================================
