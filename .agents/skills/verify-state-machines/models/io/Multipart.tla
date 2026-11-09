--------------------------- MODULE Multipart ---------------------------
EXTENDS Naturals
CONSTANT Mutant
VARIABLES producer, consumer, reader, file, writer, failed
vars == <<producer, consumer, reader, file, writer, failed>>
Init == /\ producer = "copy" /\ consumer = "request" /\ reader = "open"
        /\ file = "open" /\ writer = "open" /\ failed = FALSE
ProducerCopies == /\ producer = "copy" /\ reader = "open" /\ consumer = "request"
                  /\ producer' = "cleanup" /\ UNCHANGED <<consumer,reader,file,writer,failed>>
RequestFails == /\ consumer = "request" /\ consumer' = "return"
                /\ failed' = TRUE /\ UNCHANGED <<producer,reader,file,writer>>
ResponseRead == /\ consumer = "request"
                /\ consumer' = "return" /\ UNCHANGED <<producer,reader,file,writer,failed>>
CloseReader == /\ consumer = "return" /\ consumer' = "done"
               /\ reader' = IF Mutant = 1 THEN "open" ELSE "closed"
               /\ UNCHANGED <<producer,file,writer,failed>>
UnblockProducer == /\ producer = "copy" /\ reader = "closed" /\ producer' = "cleanup"
                   /\ UNCHANGED <<consumer,reader,file,writer,failed>>
CloseWriter == /\ producer = "cleanup" /\ producer' = "closefile" /\ writer' = "closed"
               /\ UNCHANGED <<consumer,reader,file,failed>>
CloseFile == /\ producer = "closefile" /\ producer' = "done"
             /\ file' = IF Mutant = 2 THEN "open" ELSE "closed"
             /\ writer' = IF Mutant = 3 THEN "open" ELSE writer
             /\ UNCHANGED <<consumer,reader,failed>>
Done == /\ consumer = "done" /\ producer = "done" /\ UNCHANGED vars
Next == ProducerCopies \/ RequestFails \/ ResponseRead \/ CloseReader \/ UnblockProducer \/ CloseFile \/ CloseWriter \/ Done
Spec == Init /\ [][Next]_vars
ReaderClosedOnReturn == consumer = "done" => reader = "closed"
ProducerOwnsCleanup == producer = "done" => file = "closed" /\ writer = "closed"
WriterBeforeFile == file = "closed" => writer = "closed"
ReturnMeansFileClosed == consumer = "done" => file = "closed"
=============================================================================
