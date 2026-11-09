---------------------------- MODULE Download ----------------------------
EXTENDS Naturals
CONSTANT Mutant
VARIABLES phase, mode, destination, temporary, closed, result, cancelled, shortWrite
vars == <<phase,mode,destination,temporary,closed,result,cancelled,shortWrite>>
Init == /\ phase = "start" /\ mode = "none" /\ destination = "old" /\ temporary = "absent"
        /\ closed = FALSE /\ result = "none" /\ cancelled = FALSE /\ shortWrite = FALSE
Choose == /\ phase = "start" /\ mode' \in {"download","export","stdout"} /\ phase' = "copy"
          /\ temporary' = IF mode' = "stdout" THEN "absent" ELSE "empty"
          /\ destination' = IF Mutant = 2 /\ mode' # "stdout" THEN "empty" ELSE destination
          /\ UNCHANGED <<closed,result,cancelled,shortWrite>>
Copy == /\ phase = "copy" /\ phase' = "close"
        /\ temporary' \in {"partial","complete"} /\ shortWrite' \in BOOLEAN
        /\ UNCHANGED <<mode,destination,closed,result,cancelled>>
Close == /\ phase = "close" /\ phase' = "publish" /\ closed' \in BOOLEAN
         /\ cancelled' \in BOOLEAN /\ UNCHANGED <<mode,destination,temporary,result,shortWrite>>
Publish == \E renameOK \in BOOLEAN : /\ phase = "publish" /\ phase' = "done"
           /\ LET complete == temporary = "complete" /\ ~shortWrite
                  allowed == complete /\ (mode = "stdout" \/ (closed /\ renameOK)) /\ (mode # "export" \/ ~cancelled)
                  success == allowed \/ Mutant = 1 \/ (Mutant = 3 /\ mode = "stdout" /\ shortWrite)
              IN /\ result' = IF success THEN "success" ELSE "error"
                 /\ destination' = IF mode # "stdout" /\ (allowed \/ Mutant = 4) THEN temporary ELSE destination
           /\ UNCHANGED <<mode,temporary,closed,cancelled,shortWrite>>
Done == /\ phase = "done" /\ UNCHANGED vars
Next == Choose \/ Copy \/ Close \/ Publish \/ Done
Spec == Init /\ [][Next]_vars
SuccessNeedsComplete == result = "success" => temporary = "complete" /\ ~shortWrite
FailurePreservesDestination == mode # "stdout" /\ result = "error" => destination = "old"
StdoutShortWriteFails == mode = "stdout" /\ shortWrite /\ phase = "done" => result = "error"
PublicationAfterClose == mode # "stdout" /\ destination # "old" => closed /\ temporary = "complete" /\ ~shortWrite /\ (mode # "export" \/ ~cancelled)
=============================================================================
