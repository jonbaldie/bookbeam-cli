-------------------------- MODULE AuthPoll --------------------------
EXTENDS Naturals
CONSTANT Mutation
VARIABLES phase, now, interval, polls, token, saved, reported, checked, pendingInterval, cancelled, saveChecked
vars == <<phase, now, interval, polls, token, saved, reported, checked, pendingInterval, cancelled, saveChecked>>
Init == /\ phase = "Check deadline" /\ now = 0 /\ interval = 1
        /\ polls = 0 /\ token = "absent" /\ saved = FALSE /\ reported = FALSE
        /\ checked = FALSE /\ pendingInterval = 1 /\ cancelled = FALSE /\ saveChecked = FALSE
CheckDeadline == /\ phase = "Check deadline" /\ now < 2 /\ polls < 2
                 /\ phase' = "Sleep" /\ checked' = TRUE
                 /\ UNCHANGED <<now, interval, polls, token, saved, reported, pendingInterval, cancelled, saveChecked>>
Expire == /\ phase = "Check deadline" /\ (now >= 2 \/ polls >= 2)
          /\ phase' = "Timed out" /\ UNCHANGED <<now, interval, polls, token, saved, reported, checked, pendingInterval, cancelled, saveChecked>>
Sleep == /\ phase = "Sleep" /\ (~cancelled \/ Mutation = 3) /\ now' = now + interval /\ phase' = "Request token"
         /\ UNCHANGED <<interval, polls, token, saved, reported, checked, pendingInterval, cancelled, saveChecked>>
RequestToken == /\ phase = "Request token" /\ phase' = "Read response" /\ polls' = polls + 1
                /\ UNCHANGED <<now, interval, token, saved, reported, checked, pendingInterval, cancelled, saveChecked>>
Pending == /\ phase = "Read response" /\ phase' = "Check deadline" /\ checked' = FALSE
           /\ pendingInterval' = interval
           /\ interval' = IF Mutation = 1 THEN interval + 1 ELSE interval
           /\ UNCHANGED <<now, polls, token, saved, reported, cancelled, saveChecked>>
SlowDown == /\ phase = "Read response" /\ phase' = "Check deadline" /\ checked' = FALSE
            /\ interval' = interval + 5 /\ pendingInterval' = interval + 5
            /\ UNCHANGED <<now, polls, token, saved, reported, cancelled, saveChecked>>
TokenResponse == /\ phase = "Read response" /\ token' \in {"empty", "nonempty"}
                 /\ phase' = "Check cancellation"
                 /\ UNCHANGED <<now, interval, polls, saved, reported, checked, pendingInterval, cancelled, saveChecked>>
ResponseError == /\ phase = "Read response" /\ phase' = "Failed"
                 /\ UNCHANGED <<now, interval, polls, token, saved, reported, checked, pendingInterval, cancelled, saveChecked>>
SaveToken == /\ phase = "Save token" /\ phase' = "Report success" /\ saved' = TRUE
             /\ UNCHANGED <<now, interval, polls, token, reported, checked, pendingInterval, cancelled, saveChecked>>
SaveFailure == /\ phase = "Save token" /\ phase' = "Failed"
               /\ reported' = (Mutation = 2)
               /\ UNCHANGED <<now, interval, polls, token, saved, checked, pendingInterval, cancelled, saveChecked>>
Report == /\ phase = "Report success" /\ phase' = "Logged in" /\ reported' = TRUE
          /\ UNCHANGED <<now, interval, polls, token, saved, checked, pendingInterval, cancelled, saveChecked>>
Cancel == /\ ~cancelled /\ phase \in {"Sleep", "Check cancellation"} /\ cancelled' = TRUE
          /\ UNCHANGED <<phase, now, interval, polls, token, saved, reported, checked, pendingInterval, saveChecked>>
StopCancelled == /\ cancelled /\ phase \in {"Sleep", "Check cancellation"}
                 /\ phase' = "Cancelled"
                 /\ UNCHANGED <<now, interval, polls, token, saved, reported, checked, pendingInterval, cancelled, saveChecked>>
CheckCancellation == /\ phase = "Check cancellation" /\ (~cancelled \/ Mutation = 4)
                     /\ saveChecked' = ~cancelled /\ phase' = "Save token"
                     /\ UNCHANGED <<now, interval, polls, token, saved, reported, checked, pendingInterval, cancelled>>
UnauthorizedResponse == /\ phase = "Read response"
                        /\ phase' = IF Mutation = 5 THEN "Retry unauthorized" ELSE "Failed"
                        /\ UNCHANGED <<now, interval, polls, token, saved, reported, checked, pendingInterval, cancelled, saveChecked>>
RetryUnauthorized == /\ phase = "Retry unauthorized" /\ phase' = "Check deadline" /\ checked' = FALSE
                     /\ UNCHANGED <<now, interval, polls, token, saved, reported, pendingInterval, cancelled, saveChecked>>
Done == /\ phase \in {"Logged in", "Failed", "Timed out", "Cancelled"} /\ UNCHANGED vars
Next == CheckDeadline \/ Expire \/ Sleep \/ RequestToken \/ Pending \/ SlowDown \/ TokenResponse \/ ResponseError \/ UnauthorizedResponse \/ RetryUnauthorized \/ SaveToken \/ SaveFailure \/ Report \/ Cancel \/ StopCancelled \/ CheckCancellation \/ Done
Spec == Init /\ [][Next]_vars
BackoffOnlyOnSlowDown == phase = "Check deadline" => interval = pendingInterval
ReportRequiresSave == reported => saved
NoRequestAfterExpiry == phase = "Read response" => now < 2
LoginHasToken == reported => token = "nonempty"
WaitCancellationStopsPolling == phase = "Request token" => ~cancelled
SaveRequiresCancellationCheck == saved => saveChecked
UnauthorizedIsTerminal == phase # "Retry unauthorized"
====================================================================
