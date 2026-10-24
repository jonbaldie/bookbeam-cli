#!/usr/bin/env python3
"""Interrupt `bookbeam files download` mid-transfer, as Ctrl-C would, and log what is left on disk.

usage: interrupt.py <project-id> <file-id> <dest> <reference-file> <delay-seconds> [signal]
Reads the token like bb.py; appends a redacted record to session.log.
"""
import hashlib, os, signal, subprocess, sys, time
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))
import bb  # noqa: E402

project, file_id, dest, ref, delay = sys.argv[1:6]
signame = sys.argv[6] if len(sys.argv) > 6 else "SIGINT"
dest_path = Path(dest)
def state(p):
    if not p.exists():
        return "absent"
    data = p.read_bytes()
    return f"{len(data)} bytes sha256={hashlib.sha256(data).hexdigest()[:16]}"
ref_state = state(Path(ref))
before = state(dest_path)
env = os.environ.copy()
env["BOOKBEAM_TOKEN"] = bb.TOKEN
env["BOOKBEAM_CONFIG_DIR"] = str(bb.ROOT / ".exploratory-run" / "configs" / "live")
env.pop("BOOKBEAM_HOST", None)
args = ["files", "download", project, file_id, "-o", dest]
proc = subprocess.Popen([str(bb.BIN), *args], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                        env=env, cwd=str(bb.ROOT / ".exploratory-run" / "scratch"))
time.sleep(float(delay))
mid = state(dest_path)
proc.send_signal(getattr(signal, signame))
out, err = proc.communicate(timeout=60)
after = state(dest_path)
record = (f"\n===== interrupted download ({signame} after {delay}s) =====\n$ bookbeam {' '.join(args)}\n"
          f"reference: {ref_state}\ndest before: {before}\ndest at signal: {mid}\nexit={proc.returncode}\n"
          f"--- stdout ---\n{out}\n--- stderr ---\n{err}\ndest after: {after}\n")
record = bb.redact(record)
with bb.LOG.open("a") as fh:
    fh.write(record)
print(record)
