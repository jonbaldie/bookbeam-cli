-------------------------- MODULE RemoteRequests --------------------------
EXTENDS TLC
CONSTANTS Mutant, WebhookMutant
VARIABLES operation, phase, valid, responseOK, requested, reported, urlSupplied
vars == <<operation, phase, valid, responseOK, requested, reported, urlSupplied>>
Operations == {"provider-configure", "provider-disconnect", "provider-lists", "webhook-set", "webhook-clear", "catalog-create", "catalog-update", "catalog-delete", "billing-checkout"}
Init == /\ operation \in Operations /\ phase = "validate"
        /\ valid \in BOOLEAN /\ responseOK \in BOOLEAN
        /\ requested = FALSE /\ reported = FALSE /\ urlSupplied \in BOOLEAN
IsWebhook == operation \in {"webhook-set", "webhook-clear"}
Validated == valid /\ (~IsWebhook \/ urlSupplied \/ WebhookMutant)
Validate == /\ phase = "validate" /\ phase' = IF Validated THEN "request" ELSE "error"
            /\ UNCHANGED <<operation, valid, responseOK, requested, reported, urlSupplied>>
Request == /\ phase = "request" /\ requested' = TRUE /\ phase' = "response"
           /\ UNCHANGED <<operation, valid, responseOK, reported, urlSupplied>>
Report == /\ phase = "response" /\ phase' = IF responseOK THEN "done" ELSE "error"
          /\ reported' = responseOK /\ UNCHANGED <<operation, valid, responseOK, requested, urlSupplied>>
PrematureReport == /\ Mutant /\ phase = "validate" /\ reported' = TRUE /\ phase' = "done"
                   /\ UNCHANGED <<operation, valid, responseOK, requested, urlSupplied>>
Done == /\ phase \in {"done", "error"} /\ UNCHANGED vars
Next == Validate \/ Request \/ Report \/ PrematureReport \/ Done
ReportAfterRequest == reported => (requested /\ valid /\ responseOK)
ExplicitWebhookURL == (requested /\ IsWebhook) => urlSupplied
ErrorMeansNoMutationRequest == (phase = "error" /\ operation = "catalog-create") => ~requested
Spec == Init /\ [][Next]_vars
=============================================================================
