#!/bin/bash
# Replay driver used for this pass: runs bookbeam, appends the command, stdout,
# stderr and exit code to $EVIDENCE_DIR/session.log.
# usage: BOOKBEAM=./bookbeam EVIDENCE_DIR=. ./r.sh <args...>
log=${EVIDENCE_DIR:-.}/session.log
err=$(mktemp)
echo "\$ bookbeam $*" >> "$log"
out=$("${BOOKBEAM:-bookbeam}" "$@" 2>"$err"); rc=$?
echo "$out" | tee -a "$log"
sed 's/^/[stderr] /' "$err" | tee -a "$log"
echo "[exit $rc]" | tee -a "$log"
echo >> "$log"
rm -f "$err"
