-------------------------- MODULE Readiness --------------------------
CONSTANT Mutation
VARIABLES phase, settings, client, printer, ran, resolutionFailed, public, hasToken
vars == <<phase, settings, client, printer, ran, resolutionFailed, public, hasToken>>
Init == /\ phase = "Resolve config" /\ settings = FALSE /\ client = FALSE
        /\ printer = FALSE /\ ran = FALSE /\ resolutionFailed = FALSE
        /\ public \in BOOLEAN /\ hasToken \in BOOLEAN
Resolve == /\ phase = "Resolve config" /\ phase' = "Install settings"
           /\ UNCHANGED <<settings, client, printer, ran, resolutionFailed, public, hasToken>>
ResolveFailure == /\ phase = "Resolve config" /\ resolutionFailed' = TRUE
                  /\ phase' = IF Mutation = 1 THEN "Run command" ELSE "Config error"
                  /\ UNCHANGED <<settings, client, printer, ran, public, hasToken>>
Settings == /\ phase = "Install settings" /\ settings' = TRUE /\ phase' = "Install client"
            /\ UNCHANGED <<client, printer, ran, resolutionFailed, public, hasToken>>
Client == /\ phase = "Install client" /\ client' = TRUE /\ phase' = "Install printer"
          /\ UNCHANGED <<settings, printer, ran, resolutionFailed, public, hasToken>>
Printer == /\ phase = "Install printer" /\ printer' = TRUE /\ phase' = "Authenticate"
           /\ UNCHANGED <<settings, client, ran, resolutionFailed, public, hasToken>>
Run == /\ phase = "Run command" /\ ran' = TRUE /\ phase' = "Command returned"
       /\ UNCHANGED <<settings, client, printer, resolutionFailed, public, hasToken>>
Authenticate == /\ phase = "Authenticate"
                /\ phase' = IF public \/ hasToken \/ Mutation = 2 THEN "Run command" ELSE "Auth error"
                /\ UNCHANGED <<settings, client, printer, ran, resolutionFailed, public, hasToken>>
Done == /\ phase \in {"Config error", "Auth error", "Command returned"} /\ UNCHANGED vars
Next == Resolve \/ ResolveFailure \/ Settings \/ Client \/ Printer \/ Authenticate \/ Run \/ Done
Spec == Init /\ [][Next]_vars
CommandRequiresReady == ran => (settings /\ client /\ printer /\ ~resolutionFailed)

CommandRequiresAuthentication == ran => (public \/ hasToken)
====================================================================
