#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import urllib.request

SKILL = Path(__file__).resolve().parents[1]
REPO = SKILL.parents[2]
VERSION = "1.7.4"
SHA256 = "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88"


def run(args, cwd, logfile, timeout=180):
    result = subprocess.run(args, cwd=cwd, text=True, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT, timeout=timeout)
    logfile.write_text(result.stdout)
    return result


def source_check():
    record = json.loads((SKILL / "references/source.json").read_text())
    changed = [name for name, digest in record["files"].items()
               if not (REPO / name).is_file()
               or hashlib.sha256((REPO / name).read_bytes()).hexdigest() != digest]
    current = set(p.relative_to(REPO).as_posix() for p in REPO.rglob("*.go")
                  if not p.name.endswith("_test.go") and ".git" not in p.parts)
    recorded = {name for name in record["files"] if name.endswith(".go")}
    changed += sorted(current ^ recorded)
    if changed:
        raise RuntimeError("Source needs model review: " + ", ".join(sorted(set(changed))))
    return record["commit"]


def get_jar(given):
    jar = Path(given).resolve() if given else Path.home() / ".cache/bookbeam-formal/tla2tools-1.7.4.jar"
    if not jar.exists():
        jar.parent.mkdir(parents=True, exist_ok=True)
        with urllib.request.urlopen(f"https://github.com/tlaplus/tlaplus/releases/download/v{VERSION}/tla2tools.jar") as response:
            data = response.read()
        if hashlib.sha256(data).hexdigest() != SHA256:
            raise RuntimeError("Downloaded TLC checksum mismatch")
        jar.write_bytes(data)
    if hashlib.sha256(jar.read_bytes()).hexdigest() != SHA256:
        raise RuntimeError("TLC checksum mismatch")
    return jar


def models(jar, out):
    results = []
    for area in sorted((SKILL / "models").iterdir()):
        manifest = area / "manifest.json"
        if not manifest.is_file():
            continue
        entries = json.loads(manifest.read_text())
        baseline_rules = {(entry["model"], rule)
                          for entry in entries if entry["kind"] == "baseline"
                          for rule in re.findall(r"^INVARIANT\s+(\w+)",
                                                 (area / entry["config"]).read_text(), re.M)}
        mutant_rules = {(entry["model"], entry["invariant"])
                        for entry in entries if entry["kind"] == "mutant"}
        if not baseline_rules or baseline_rules - mutant_rules:
            raise RuntimeError(f"Baseline rules lack fault sensitivity in {area.name}")
        for entry in entries:
            config = entry["config"]
            name = area.name + "-" + Path(config).stem
            work = out / name
            work.mkdir()
            for source in area.glob("*.tla"):
                shutil.copy2(source, work / source.name)
            shutil.copy2(area / config, work / config)
            result = run([os.environ.get("JAVA", "java"), "-XX:+UseParallelGC", "-cp", str(jar),
                          "tlc2.TLC", "-workers", "1", "-fp", "0", "-config", config,
                          entry["model"]], work, work / "tlc.out")
            if entry["expectation"] == "pass":
                valid = result.returncode == 0 and "Model checking completed. No error has been found." in result.stdout
            else:
                valid = result.returncode != 0 and f'Invariant {entry["invariant"]} is violated' in result.stdout
            if not valid:
                raise RuntimeError(f"Unexpected model result: {name}. Read {work / 'tlc.out'}")
            match = re.search(r"([\d,]+) states generated, ([\d,]+) distinct states found", result.stdout)
            results.append(dict(entry, area=area.name, states=match.group(2) if match else None))
            print(f"{entry['kind']:8s} {name}: expected {entry['expectation']}", flush=True)
    if not results:
        raise RuntimeError("No models registered")
    return results


def replay(out):
    with tempfile.TemporaryDirectory(prefix="bookbeam-formal-") as tmp:
        copy = Path(tmp) / "repo"
        shutil.copytree(REPO, copy, ignore=shutil.ignore_patterns(".git", ".agents"))
        (copy / ".bookbeam-formal-copy").touch()
        for area in (SKILL / "models").iterdir():
            mapping = area / "replay-map.json"
            if mapping.is_file():
                for source, destination in json.loads(mapping.read_text()).items():
                    shutil.copy2(area / source, copy / destination)
        result = run(["go", "test", "./cmd", "./pkg/...", "-count=1"], copy, out / "go-baseline.out")
        if result.returncode:
            raise RuntimeError(f"Go baseline failed. Read {out / 'go-baseline.out'}")
        auth_mutations = run([sys.executable, str(SKILL / "models/auth/mutate_real.py"),
                              str(copy), str(out / "auth-source-mutants")], copy,
                             out / "go-auth-source-mutants.out")
        if auth_mutations.returncode:
            raise RuntimeError("Authentication source mutation checks failed")
        io_mutations = run([sys.executable, str(SKILL / "models/io/mutate_real.py"),
                            str(copy), str(out / "io-source-mutants")], copy,
                           out / "go-io-source-mutants.out")
        if io_mutations.returncode:
            raise RuntimeError("I/O source mutation checks failed")
        target = copy / "pkg/output/printer.go"
        baseline = target.read_text()
        old = "if force || p.JSON {"
        if baseline.count(old) != 1:
            raise RuntimeError("Fault injection needs source review")
        target.write_text(baseline.replace(old, "if force {", 1))
        try:
            broken = run(["go", "test", "./pkg/output", "-run", "TestFormal", "-count=1"],
                         copy, out / "go-source-mutant.out")
            if broken.returncode == 0 or "--- FAIL: TestFormal" not in broken.stdout:
                raise RuntimeError("Real source fault was not detected by formal replay tests")
        finally:
            target.write_text(baseline)
        restored = run(["go", "test", "./pkg/output", "-run", "TestFormal", "-count=1"],
                       copy, out / "go-restored.out")
        if restored.returncode:
            raise RuntimeError("Restored source did not pass")


def main():
    parser = argparse.ArgumentParser(description="Check source, bounded models, and real Go behavior")
    parser.add_argument("--out", type=Path, help="New directory for results; default is a temporary directory")
    parser.add_argument("--jar", help="Existing checksum-pinned TLC jar")
    args = parser.parse_args()
    out = args.out.resolve() if args.out else Path(tempfile.mkdtemp(prefix="bookbeam-formal-results-"))
    if args.out:
        out.mkdir(parents=True, exist_ok=False)
    print(f"Results: {out}", flush=True)
    commit = source_check()
    results = models(get_jar(args.jar), out)
    replay(out)
    summary = {"reviewed_commit": commit, "tla2tools_release": VERSION,
               "tlc_version": "2.19", "checks": results,
               "go_replays": "pass", "real_source_mutation": "detected and restored"}
    (out / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(f"Verified {len(results)} bounded checks, Go replays, and source fault detection.")


if __name__ == "__main__":
    main()
