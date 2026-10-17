#!/usr/bin/env python3
"""Journey 2: publish a disposable book with a cover, file, and signup link."""

import json
import sys
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import bb

ROOT = Path("/Users/jonathanbaldie/Code-2/github.com/jonbaldie/bookbeam-cli")
LIVE = ROOT / ".exploratory-run" / "configs" / "live"
FIX = ROOT / ".exploratory-run" / "scratch" / "fixtures"
SCRATCH = ROOT / ".exploratory-run" / "scratch"


def note(msg):
    with bb.LOG.open("a") as fh:
        fh.write(f"\n# NOTE {msg}\n")
    print(f"\n# NOTE {msg}\n")


def last_json():
    text = bb.LOG.read_text()
    marker = "--- stdout ---\n"
    end = "\n--- stderr ---"
    idx = text.rfind(marker)
    raw = text[idx + len(marker): text.find(end, idx)].strip()
    return json.loads(raw)


def main():
    note("journey 2 start")
    bb.run(
        ["projects", "create", "--title", "Exploratory 2026-10-17", "--description", "Disposable cover and file journey", "--cover", str(FIX / "cover-red.png"), "--json"],
        config_dir=LIVE,
        label="create project with cover",
        timeout=60,
    )
    created = last_json()["data"]
    project_id = created["id"]
    note(f"created id={project_id} cover_url_set={bool(created.get('cover_image_url'))} description={created.get('description')!r}")

    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get created project")
    got = last_json()["data"]
    note(f"get title={got.get('title')!r} description={got.get('description')!r} cover_set={bool(got.get('cover_image_url'))}")

    cover_url = got.get("cover_image_url")
    if cover_url:
        req = urllib.request.Request(cover_url, method="GET")
        with urllib.request.urlopen(req, timeout=30) as resp:
            body = resp.read()
            note(f"cover fetch status={resp.status} type={resp.headers.get('content-type')} bytes={len(body)} magic={body[:8]!r}")

    bb.run(
        ["projects", "update", str(project_id), "--title", "Exploratory 2026-10-17 renamed", "--json"],
        config_dir=LIVE,
        label="rename only",
    )
    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get after rename")
    got = last_json()["data"]
    note(f"after rename title={got.get('title')!r} description={got.get('description')!r} cover_set={bool(got.get('cover_image_url'))}")

    bb.run(["projects", "update", str(project_id)], config_dir=LIVE, label="update with no flags")
    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get after empty update")
    got = last_json()["data"]
    note(f"after empty update title={got.get('title')!r} description={got.get('description')!r}")

    bb.run(
        ["projects", "update", str(project_id), "--cover", str(FIX / "cover-blue.png"), "--remove-cover"],
        config_dir=LIVE,
        label="conflicting cover flags",
    )
    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get after conflicting cover flags")
    got = last_json()["data"]
    note(f"after conflict cover_set={bool(got.get('cover_image_url'))} url_changed={got.get('cover_image_url')!=cover_url}")

    bb.run(
        ["projects", "update", str(project_id), "--cover", str(FIX / "cover-blue.png"), "--json"],
        config_dir=LIVE,
        label="replace cover",
        timeout=60,
    )
    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get after cover replace")
    got = last_json()["data"]
    new_cover = got.get("cover_image_url")
    note(f"replaced cover_set={bool(new_cover)} changed={new_cover!=cover_url} description={got.get('description')!r}")
    if new_cover:
        with urllib.request.urlopen(new_cover, timeout=30) as resp:
            body = resp.read()
            note(f"new cover fetch status={resp.status} type={resp.headers.get('content-type')} bytes={len(body)} magic={body[:8]!r}")

    bb.run(
        ["projects", "update", str(project_id), "--clear-description", "--description", "Conflict"],
        config_dir=LIVE,
        label="conflicting description flags",
    )
    bb.run(
        ["projects", "update", str(project_id), "--remove-cover", "--json"],
        config_dir=LIVE,
        label="remove cover",
    )
    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get after remove cover")
    got = last_json()["data"]
    note(f"after remove cover={got.get('cover_image_url')!r} description={got.get('description')!r} title={got.get('title')!r}")

    bb.run(
        ["files", "upload", str(project_id), str(FIX / "notes.txt")],
        config_dir=LIVE,
        label="reject non-book extension",
    )
    bb.run(["files", "list", str(project_id), "--json"], config_dir=LIVE, label="files after rejected upload")
    note(f"files after reject count={len(last_json().get('data') or [])}")

    bb.run(
        ["files", "upload", str(project_id), str(FIX / "two words.epub"), "--json"],
        config_dir=LIVE,
        label="upload spaced epub",
        timeout=60,
    )
    uploaded = last_json()["data"]
    file_id = uploaded["id"]
    note(f"uploaded file id={file_id} filename={uploaded.get('filename')!r} size={uploaded.get('file_size')}")

    dest = SCRATCH / "downloaded.epub"
    if dest.exists():
        dest.unlink()
    bb.run(
        ["files", "download", str(project_id), str(file_id), "-o", str(dest), "--json"],
        config_dir=LIVE,
        label="download explicit output",
        timeout=60,
    )
    original = (FIX / "two words.epub").read_bytes()
    downloaded = dest.read_bytes() if dest.exists() else b""
    note(f"explicit download exists={dest.exists()} bytes={len(downloaded)} match={downloaded==original}")

    # negotiated name, from scratch cwd
    negotiated = SCRATCH / "two words.epub"
    if negotiated.exists():
        negotiated.unlink()
    bb.run(
        ["files", "download", str(project_id), str(file_id), "--json"],
        config_dir=LIVE,
        label="download negotiated filename",
        timeout=60,
    )
    note(f"negotiated exists={negotiated.exists()} match={negotiated.exists() and negotiated.read_bytes()==original}")

    bb.run(
        ["files", "delete", str(project_id), str(file_id)],
        config_dir=LIVE,
        stdin="n\n",
        label="cancel file delete",
    )
    bb.run(["files", "list", str(project_id), "--json"], config_dir=LIVE, label="files after cancel delete")
    note(f"files after cancel={[item.get('id') for item in last_json().get('data') or []]}")

    bb.run(
        ["links", "create", str(project_id), "--title", "Sample Chapter", "--consent", "Join my newsletter", "--json"],
        config_dir=LIVE,
        label="create signup link",
    )
    link = last_json()["data"]
    link_id = link["id"]
    slug = link.get("slug")
    note(f"link id={link_id} slug={slug} title={link.get('title')!r} consent={link.get('opt_in_text')!r}")

    bb.run(["links", "list", str(project_id)], config_dir=LIVE, label="links list table")
    bb.run(["links", "list", str(project_id), "--json"], config_dir=LIVE, label="links list json")
    bb.run(
        ["--host", "https://bookbeam.app/", "links", "list", str(project_id)],
        config_dir=LIVE,
        label="links list trailing-slash host",
    )
    if slug:
        url = f"https://bookbeam.app/download/{slug}"
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=30) as resp:
            html = resp.read(500)
            note(f"public page status={resp.status} type={resp.headers.get('content-type')} starts={html[:80]!r}")

    bb.run(
        ["links", "update", str(project_id), str(link_id), "--title", "Updated Chapter", "--json"],
        config_dir=LIVE,
        label="link title-only update",
    )
    bb.run(["links", "list", str(project_id), "--json"], config_dir=LIVE, label="links after title update")
    rows = last_json().get("data") or []
    row = next(item for item in rows if item.get("id") == link_id)
    note(f"after title update title={row.get('title')!r} consent={row.get('opt_in_text')!r}")

    bb.run(
        ["links", "update", str(project_id), str(link_id), "--consent", "New consent", "--clear-consent"],
        config_dir=LIVE,
        label="conflicting consent flags",
    )
    bb.run(
        ["links", "update", str(project_id), str(link_id), "--clear-consent", "--json"],
        config_dir=LIVE,
        label="clear consent",
    )
    bb.run(["links", "list", str(project_id), "--json"], config_dir=LIVE, label="links after clear consent")
    rows = last_json().get("data") or []
    row = next(item for item in rows if item.get("id") == link_id)
    note(f"after clear title={row.get('title')!r} consent={row.get('opt_in_text')!r}")

    bb.run(
        ["links", "delete", str(project_id), str(link_id)],
        config_dir=LIVE,
        stdin="n\n",
        label="cancel link delete",
    )
    bb.run(["links", "list", str(project_id), "--json"], config_dir=LIVE, label="links after cancel delete")
    note(f"links after cancel={[item.get('id') for item in last_json().get('data') or []]}")

    # Route the disposable project only. Do not change the team provider.
    bb.run(
        ["projects", "newsletter", str(project_id), "--list-id", "f360d073-6b24-404e-bb01-c8d9d1e1cf16", "--tags", "sci-fi, readers ,et", "--json"],
        config_dir=LIVE,
        label="set newsletter routing",
    )
    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get after newsletter route")
    got = last_json()["data"]
    note(f"routing list={got.get('newsletter_list_id')!r} tags={got.get('newsletter_tags')!r}")

    bb.run(
        ["projects", "newsletter", str(project_id), "--clear-tags", "--json"],
        config_dir=LIVE,
        label="clear newsletter tags",
    )
    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get after clear tags")
    got = last_json()["data"]
    note(f"after clear tags list={got.get('newsletter_list_id')!r} tags={got.get('newsletter_tags')!r}")

    bb.run(
        ["projects", "delete", str(project_id)],
        config_dir=LIVE,
        stdin="n\n",
        label="cancel project delete",
    )
    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get after cancel project delete")
    note(f"still present id={last_json()['data']['id']}")

    bb.run(["projects", "delete", str(project_id), "--force", "--json"], config_dir=LIVE, label="force delete project")
    bb.run(["projects", "get", str(project_id), "--json"], config_dir=LIVE, label="get deleted project")
    bb.run(["projects", "list", "--json"], config_dir=LIVE, label="catalog after delete")
    catalog = last_json()
    note(f"catalog total={catalog.get('total')} ids={[item.get('id') for item in catalog.get('data') or []]}")
    note("journey 2 commands done")


if __name__ == "__main__":
    main()
