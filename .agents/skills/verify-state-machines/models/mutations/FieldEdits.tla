---------------------------- MODULE FieldEdits ----------------------------
EXTENDS TLC
CONSTANT Mutant
VARIABLES edit, phase, json, form
vars == <<edit, phase, json, form>>
Init == /\ edit \in {"unset", "set", "clear"}
        /\ phase = "input" /\ json = "unwritten" /\ form = "unwritten"
Encode == /\ phase = "input"
          /\ json' = IF edit = "clear" THEN (IF Mutant THEN "omitted" ELSE "null")
                     ELSE IF edit = "set" THEN "value" ELSE "omitted"
          /\ form' = IF edit = "clear" THEN "empty" ELSE IF edit = "set" THEN "value" ELSE "omitted"
          /\ phase' = "done" /\ UNCHANGED edit
Done == /\ phase = "done" /\ UNCHANGED vars
Next == Encode \/ Done
EditSemantics == phase = "done" =>
   /\ json = CASE edit = "unset" -> "omitted" [] edit = "set" -> "value" [] OTHER -> "null"
   /\ form = CASE edit = "unset" -> "omitted" [] edit = "set" -> "value" [] OTHER -> "empty"
Spec == Init /\ [][Next]_vars
=============================================================================
