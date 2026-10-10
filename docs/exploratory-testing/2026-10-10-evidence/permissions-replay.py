#!/usr/bin/env python3
"""No network or real credentials needed. Run from the repo root."""
import json
import os
from pathlib import Path
import stat
import subprocess

root = Path.cwd()
work = root / '.exploratory-run'
binary = work / 'bookbeam'
env = os.environ.copy()
for key in ('BOOKBEAM_HOST', 'BOOKBEAM_TOKEN', 'BOOKBEAM_CONFIG_DIR'):
    env.pop(key, None)
for attempt in (1, 2):
    directory = work / f'restored-config-{attempt}'
    directory.mkdir(exist_ok=True)
    directory.chmod(0o755)
    config = directory / 'config.json'
    config.write_text('{"host":"https://bookbeam.app"}')
    config.chmod(0o644)
    print(f'REPLAY {attempt}: before directory={stat.S_IMODE(directory.stat().st_mode):04o} file={stat.S_IMODE(config.stat().st_mode):04o}')
    command = [str(binary), 'auth', 'login', '--token', 'fake-permission-replay-token', '--json']
    print('$ bookbeam auth login --token fake-permission-replay-token --json')
    result = subprocess.run(command, env=env | {'BOOKBEAM_CONFIG_DIR': str(directory)}, text=True, capture_output=True, timeout=10)
    print(f'stdout={result.stdout.strip()}\nstderr={result.stderr.strip()}\nexit={result.returncode}')
    mode = stat.S_IMODE(config.stat().st_mode)
    directory_mode = stat.S_IMODE(directory.stat().st_mode)
    print(f'after directory={directory_mode:04o} file={mode:04o}; token saved={json.loads(config.read_text()).get("token") == "fake-permission-replay-token"}')
    print(f'owner-only file expectation: {"PASS" if mode == 0o600 else "FAIL"}')
    print(f'other-user directory traversal and config-read bits: {bool(directory_mode & 0o001 and mode & 0o004)}')
    config.unlink()
    directory.rmdir()
