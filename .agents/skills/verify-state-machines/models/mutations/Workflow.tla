----------------------------- MODULE Workflow -----------------------------
EXTENDS Integers, TLC
CONSTANT Mutant, CIMutant
VARIABLES workflow, phase, passed, packages, published
vars == <<workflow, phase, passed, packages, published>>
Init == /\ workflow \in {"ci-linux", "ci-macos", "ci-windows", "release"}
        /\ phase = "checkout" /\ passed = {} /\ packages = 0 /\ published = FALSE
Advance(stage, next) == /\ phase = stage /\ phase' = next /\ passed' = passed \cup {stage}
                        /\ UNCHANGED <<workflow, packages, published>>
Checkout == Advance("checkout", "setup")
Setup == Advance("setup", "test")
Test == Advance("test", "vet")
Vet == Advance("vet", "build")
BuildCI == /\ workflow # "release" /\ (phase = "build" \/ (CIMutant /\ phase = "test")) /\ phase' = "done"
           /\ passed' = passed \cup {"build"} /\ UNCHANGED <<workflow, packages, published>>
Package == /\ workflow = "release" /\ phase = "build" /\ packages < 5
           /\ packages' = packages + 1 /\ phase' = IF packages' = 5 THEN "checksums" ELSE "build"
           /\ UNCHANGED <<workflow, passed, published>>
Checksums == Advance("checksums", "publish")
Publish == /\ workflow = "release" /\ (phase = "publish" \/ (Mutant /\ phase = "build"))
           /\ published' = TRUE /\ phase' = "done" /\ UNCHANGED <<workflow, passed, packages>>
Fail == /\ phase # "done" /\ phase' = "done" /\ UNCHANGED <<workflow, passed, packages, published>>
Done == /\ phase = "done" /\ UNCHANGED vars
Next == Checkout \/ Setup \/ Test \/ Vet \/ BuildCI \/ Package \/ Checksums \/ Publish \/ Fail \/ Done
ReleaseGate == published => (workflow = "release" /\ {"checkout", "setup", "test", "vet", "checksums"} \subseteq passed /\ packages = 5)
CIBuildGate == (workflow # "release" /\ "build" \in passed) => {"checkout", "setup", "test", "vet"} \subseteq passed
Spec == Init /\ [][Next]_vars
=============================================================================
