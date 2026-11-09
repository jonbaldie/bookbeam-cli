-------------------------- MODULE Credentials --------------------------
CONSTANT Mutation
VARIABLES phase, memory, disk, override, client, writeValue, privateTemp, saveOK
vars == <<phase, memory, disk, override, client, writeValue, privateTemp, saveOK>>
Init == /\ phase = "Loaded" /\ memory = "old" /\ disk = "old"
        /\ override \in {"none", "env", "flag"} /\ client = IF override = "none" THEN "old" ELSE override
        /\ writeValue = "none" /\ privateTemp = FALSE /\ saveOK = FALSE
Store == /\ phase = "Loaded" /\ writeValue' \in {"empty", "new"}
         /\ memory' = IF Mutation = 3 THEN writeValue' ELSE memory
         /\ phase' = "Create parent"
         /\ UNCHANGED <<disk, override, client, privateTemp, saveOK>>
Mkdir == /\ phase = "Create parent" /\ phase' = "Marshal candidate"
         /\ UNCHANGED <<memory, disk, override, client, writeValue, privateTemp, saveOK>>
Marshal == /\ phase = "Marshal candidate" /\ phase' = "Create temporary"
           /\ UNCHANGED <<memory, disk, override, client, writeValue, privateTemp, saveOK>>
Create == /\ phase = "Create temporary" /\ phase' = "Write temporary" /\ privateTemp' = (Mutation # 2)
          /\ UNCHANGED <<memory, disk, override, client, writeValue, saveOK>>
Write == /\ phase = "Write temporary" /\ phase' = "Close temporary"
         /\ UNCHANGED <<memory, disk, override, client, writeValue, privateTemp, saveOK>>
Close == /\ phase = "Close temporary" /\ phase' = "Rename temporary"
         /\ UNCHANGED <<memory, disk, override, client, writeValue, privateTemp, saveOK>>
Rename == /\ phase = "Rename temporary" /\ phase' = "Commit memory"
          /\ disk' = IF Mutation = 1 /\ override # "none" THEN override ELSE writeValue
          /\ UNCHANGED <<memory, override, client, writeValue, privateTemp, saveOK>>
Commit == /\ phase = "Commit memory" /\ phase' = "Saved" /\ saveOK' = TRUE /\ memory' = writeValue
          /\ UNCHANGED <<disk, override, client, writeValue, privateTemp>>
Fail == /\ phase \in {"Create parent", "Create temporary", "Write temporary", "Close temporary", "Rename temporary"}
        /\ phase' = "Save failed" /\ UNCHANGED <<memory, disk, override, client, writeValue, privateTemp, saveOK>>
Done == /\ phase \in {"Saved", "Save failed"} /\ UNCHANGED vars
Next == Store \/ Mkdir \/ Marshal \/ Create \/ Write \/ Close \/ Rename \/ Commit \/ Fail \/ Done
Spec == Init /\ [][Next]_vars
OverridesNeverPersisted == saveOK => disk = writeValue
PermissionsBeforeWrite == phase = "Write temporary" => privateTemp
FailedSaveIsAtomic == phase = "Save failed" => (memory = "old" /\ disk = "old")
LogoutClearsEffectiveToken == (phase = "Saved" /\ memory = "empty") => override = "none"
ClientMatchesSettings == phase = "Saved" => client = (IF override = "none" THEN memory ELSE override)
====================================================================
