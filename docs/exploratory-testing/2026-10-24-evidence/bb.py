#!/usr/bin/env python3
"""Drive the public bookbeam binary and append a redacted session log.

The token is read from the ordinary user config and passed only as an
environment variable. It is never written to the log.
"""

import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path("/Users/jonathanbaldie/Code-2/github.com/jonbaldie/bookbeam-cli")
BIN = ROOT / ".exploratory-run" / "bookbeam"
LOG = ROOT / "docs" / "exploratory-testing" / "2026-10-24-evidence" / "session.log"
REAL_CONFIG = Path.home() / ".config" / "bookbeam" / "config.json"

SECRET_STRINGS = []


def load_token():
    data = json.loads(REAL_CONFIG.read_text())
    token = data.get("token") or ""
    if token:
        SECRET_STRINGS.append(token)
    return token


TOKEN = load_token()


def redact(text: str) -> str:
    out = text
    for secret in SECRET_STRINGS:
        if secret:
            out = out.replace(secret, "[REDACTED]")
    out = re.sub(r"\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b", "[EMAIL]", out, flags=re.I)
    out = re.sub(r"(bb_pat_[A-Za-z0-9_\-]+)", "[REDACTED]", out)
    out = re.sub(r"(Bearer\s+)[^\s\"']+", r"\1[REDACTED]", out)
    # Laravel/Sanctum-looking tokens that are not the known secret already replaced.
    out = re.sub(r"\b\d+\|[A-Za-z0-9]{20,}\b", "[REDACTED]", out)
    return out


def run(args, *, env_extra=None, stdin=None, config_dir=None, timeout=60, label=None, use_token=True):
    env = os.environ.copy()
    env.pop("BOOKBEAM_HOST", None)
    if use_token:
        env["BOOKBEAM_TOKEN"] = TOKEN
    else:
        env.pop("BOOKBEAM_TOKEN", None)
    if config_dir:
        env["BOOKBEAM_CONFIG_DIR"] = str(config_dir)
    else:
        env.pop("BOOKBEAM_CONFIG_DIR", None)
    if env_extra:
        for key, value in env_extra.items():
            if value is None:
                env.pop(key, None)
            else:
                env[key] = value
                if "TOKEN" in key or "token" in key:
                    SECRET_STRINGS.append(value)
    cmd = [str(BIN), *args]
    shown = " ".join(args)
    header = f"\n===== {label or shown} =====\n$ bookbeam {shown}\n"
    if config_dir:
        header += f"# BOOKBEAM_CONFIG_DIR={config_dir}\n"
    if env_extra:
        safe = {k: ("[REDACTED]" if "TOKEN" in k or "token" in k else v) for k, v in env_extra.items()}
        header += f"# env_extra={safe}\n"
    try:
        proc = subprocess.run(
            cmd,
            input=stdin,
            text=True,
            capture_output=True,
            env=env,
            timeout=timeout,
            cwd=str(ROOT / ".exploratory-run" / "scratch"),
        )
        body = (
            f"exit={proc.returncode}\n"
            f"--- stdout ---\n{proc.stdout}\n"
            f"--- stderr ---\n{proc.stderr}\n"
        )
        timed_out = False
    except subprocess.TimeoutExpired as exc:
        stdout = exc.stdout or ""
        stderr = exc.stderr or ""
        if isinstance(stdout, bytes):
            stdout = stdout.decode(errors="replace")
        if isinstance(stderr, bytes):
            stderr = stderr.decode(errors="replace")
        body = f"exit=TIMEOUT after {timeout}s\n--- stdout ---\n{stdout}\n--- stderr ---\n{stderr}\n"
        timed_out = True
        proc = None
    record = redact(header + body)
    LOG.parent.mkdir(parents=True, exist_ok=True)
    with LOG.open("a") as fh:
        fh.write(record)
    sys.stdout.write(record)
    return proc, timed_out


if __name__ == "__main__":
    # bb.py run <config_dir_or_-> -- bookbeam args...
    if len(sys.argv) < 5 or sys.argv[1] != "run" or sys.argv[3] != "--":
        print("usage: bb.py run <config_dir|-> -- args...", file=sys.stderr)
        sys.exit(2)
    config = None if sys.argv[2] == "-" else sys.argv[2]
    proc, _ = run(sys.argv[4:], config_dir=config, stdin=os.environ.get("BB_STDIN"),
                  use_token=os.environ.get("BB_NO_TOKEN") != "1")
    sys.exit(proc.returncode if proc else 124)
