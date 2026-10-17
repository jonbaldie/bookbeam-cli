#!/usr/bin/env python3
"""Journey 1: point the CLI at BookBeam and keep the saved token private."""

import json
import os
import stat
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import bb

ROOT = Path("/Users/jonathanbaldie/Code-2/github.com/jonbaldie/bookbeam-cli")
BASE = ROOT / ".exploratory-run" / "configs"
LIVE = BASE / "live"


def modes(path: Path):
    st = path.stat()
    return oct(st.st_mode & 0o777)


def note(msg):
    with bb.LOG.open("a") as fh:
        fh.write(f"\n# NOTE {msg}\n")
    print(f"\n# NOTE {msg}\n")


def main():
    note("journey 1 start")
    # Human whoami should not print the provider secret.
    bb.run(["whoami"], config_dir=LIVE, label="whoami table")
    bb.run(["whoami", "--quiet"], config_dir=LIVE, label="whoami quiet")

    hosts = [
        "https://bookbeam.app/",
        "HTTPS://bookbeam.app",
        "https://BookBeam.app",
        "https://bookbeam.app:443",
        "https://bookbeam.app//",
        "https://bookbeam.app/api",
        "https://bookbeam.app?x=1",
        "https://bookbeam.app#frag",
        "https://user:pass@bookbeam.app",
        "bookbeam.app",
        "127.0.0.1:9",
        "http://127.0.0.1:9",
        " https://bookbeam.app",
        "https://bookbeam.app ",
        "https://",
        "http://",
        "https://bookbeam.app/%zz",
        "https://[::1]",
        "",
    ]
    for host in hosts:
        label = f"whoami --host {host!r}"
        bb.run(["--host", host, "whoami", "--json"], config_dir=LIVE, label=label, timeout=20)

    note("BOOKBEAM_HOST trailing slash, no flag")
    bb.run(
        ["whoami", "--json"],
        config_dir=LIVE,
        env_extra={"BOOKBEAM_HOST": "https://bookbeam.app/"},
        label="BOOKBEAM_HOST trailing slash",
    )
    note("schemeless env host must fail before a request and must not require a token")
    bb.run(
        ["whoami"],
        config_dir=LIVE,
        env_extra={"BOOKBEAM_HOST": "bookbeam.app", "BOOKBEAM_TOKEN": None},
        use_token=False,
        label="schemeless BOOKBEAM_HOST no token",
    )

    # Fresh login permissions.
    fresh = BASE / "fresh-login"
    fresh.mkdir(parents=True, exist_ok=True)
    for child in fresh.iterdir():
        child.unlink()
    note("fresh login")
    bb.run(
        ["auth", "login", "--token", "fake-fresh-token", "--json"],
        config_dir=fresh,
        use_token=False,
        label="fresh auth login",
    )
    cfg = fresh / "config.json"
    note(f"fresh dir mode={modes(fresh)} file mode={modes(cfg)} exists={cfg.exists()}")
    saved = json.loads(cfg.read_text())
    note(f"fresh saved keys={sorted(saved)} token_saved={saved.get('token')=='fake-fresh-token'} host={saved.get('host')!r}")

    # One-off overrides must not persist.
    before = cfg.read_bytes()
    bb.run(
        ["--host", "http://127.0.0.1:9", "--token", "override-token", "whoami", "--json"],
        config_dir=fresh,
        use_token=False,
        label="one-off host and token override",
        timeout=15,
    )
    after = cfg.read_bytes()
    note(f"config byte-identical after overrides={before==after}")

    bb.run(
        ["whoami", "--json"],
        config_dir=fresh,
        env_extra={"BOOKBEAM_HOST": "http://127.0.0.1:9", "BOOKBEAM_TOKEN": "env-override-token"},
        use_token=False,
        label="env host and token override",
        timeout=15,
    )
    after_env = cfg.read_bytes()
    note(f"config byte-identical after env overrides={before==after_env}")

    # Logout keeps host, drops token.
    bb.run(["auth", "logout", "--json"], config_dir=fresh, use_token=False, label="logout json")
    saved = json.loads(cfg.read_text())
    note(f"after logout token={saved.get('token')!r} host={saved.get('host')!r} mode={modes(cfg)}")

    # Regression: pre-existing 0644 config must be tightened on login.
    old = BASE / "preexisting-0644"
    if old.exists():
        for child in old.iterdir():
            child.unlink()
    else:
        old.mkdir(parents=True)
    os.chmod(old, 0o755)
    (old / "config.json").write_text('{"host":"https://bookbeam.app/"}\n')
    os.chmod(old / "config.json", 0o644)
    note(f"before preexisting dir={modes(old)} file={modes(old/'config.json')}")
    bb.run(
        ["auth", "login", "--token", "fake-permission-replay-token", "--json"],
        config_dir=old,
        use_token=False,
        label="login into preexisting 0644",
    )
    note(f"after preexisting dir={modes(old)} file={modes(old/'config.json')}")
    saved = json.loads((old / "config.json").read_text())
    note(
        f"preexisting token_saved={saved.get('token')=='fake-permission-replay-token'} "
        f"host_unchanged={saved.get('host')=='https://bookbeam.app/'} raw={ {k: ('set' if k=='token' else v) for k,v in saved.items()} }"
    )

    # Schemeless --host on login must not create a config.
    bad = BASE / "schemeless-login"
    bad.mkdir(parents=True, exist_ok=True)
    for child in bad.iterdir():
        child.unlink()
    bb.run(
        ["--host", "bookbeam.app", "auth", "login", "--token", "should-not-save", "--json"],
        config_dir=bad,
        use_token=False,
        label="login with schemeless host",
    )
    note(f"schemeless login created config={ (bad/'config.json').exists() } dir_entries={list(bad.iterdir())}")

    # Stored invalid host should name the config file and block every command, including logout.
    broken = BASE / "stored-invalid-host"
    broken.mkdir(parents=True, exist_ok=True)
    (broken / "config.json").write_text('{"host":"bookbeam.app","token":"stored-token"}\n')
    os.chmod(broken / "config.json", 0o600)
    bb.run(["whoami"], config_dir=broken, use_token=False, label="stored schemeless host whoami")
    bb.run(["auth", "logout", "--json"], config_dir=broken, use_token=False, label="stored schemeless host logout")
    note(f"stored invalid config still={ (broken/'config.json').read_text()!r }")

    # Stored trailing slash should still reach the API; file should stay as the user saved it.
    slash = BASE / "stored-trailing-slash"
    slash.mkdir(parents=True, exist_ok=True)
    (slash / "config.json").write_text(json.dumps({"host": "https://bookbeam.app/", "token": "ignored"}) + "\n")
    before = (slash / "config.json").read_bytes()
    bb.run(["whoami", "--json"], config_dir=slash, label="stored trailing-slash host")
    after = (slash / "config.json").read_bytes()
    note(f"trailing-slash config unchanged={before==after}")

    # Help text for host.
    bb.run(["--help"], config_dir=LIVE, label="root help")
    bb.run(["auth", "login", "--help"], config_dir=LIVE, label="login help")
    note("journey 1 commands done")


if __name__ == "__main__":
    main()
