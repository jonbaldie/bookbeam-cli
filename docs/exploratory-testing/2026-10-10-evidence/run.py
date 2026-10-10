#!/usr/bin/env python3
"""Drive the public CLI. Live writes are confined to one disposable project.
Run from the repo root after building .exploratory-run/bookbeam.
Uses BOOKBEAM_CONFIG_DIR (or ~/.config/bookbeam) only to read credentials.
"""
import csv
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shlex
import stat
import subprocess
import traceback
import zipfile

ROOT = Path.cwd()
WORK = ROOT / '.exploratory-run'
EVIDENCE = ROOT / 'docs/exploratory-testing/2026-10-10-evidence'
BINARY = WORK / 'bookbeam'
SOURCE = Path(os.environ.get('BOOKBEAM_CONFIG_DIR', Path.home() / '.config/bookbeam')) / 'config.json'
original = SOURCE.read_bytes()
config = json.loads(original)
token = os.environ.get('BOOKBEAM_TOKEN') or config.get('token', '')
host = os.environ.get('BOOKBEAM_HOST') or config.get('host', 'https://bookbeam.app')
env = os.environ.copy()
for key in ('BOOKBEAM_TOKEN', 'BOOKBEAM_HOST', 'BOOKBEAM_CONFIG_DIR'):
    env.pop(key, None)
live = env | {'BOOKBEAM_CONFIG_DIR': str(WORK / 'live-config'), 'BOOKBEAM_TOKEN': token, 'BOOKBEAM_HOST': host}
log = (EVIDENCE / 'session.log').open('w')


def scrub(text):
    if token:
        text = text.replace(token, '[REDACTED-PAT]')
    text = re.sub(r'("(?:api_token|api_key|token|secret|password|authorization)"\s*:\s*)"[^"]*"', r'\1"[REDACTED]"', text, flags=re.I)
    return re.sub(r'[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}', '[REDACTED-EMAIL]', text)


def note(text):
    log.write(scrub(text) + '\n')
    log.flush()


def run(args, use_env=None, stdin='', required=True):
    args = list(map(str, args))
    note('$ bookbeam ' + shlex.join(args))
    try:
        p = subprocess.run([str(BINARY), *args], cwd=WORK, env=use_env or live, input=stdin, text=True, capture_output=True, timeout=90)
    except subprocess.TimeoutExpired as exc:
        note(f'TIMEOUT: {exc}')
        raise
    note(f'stdout:\n{p.stdout}\nstderr:\n{p.stderr}\nexit={p.returncode}')
    if required and p.returncode:
        raise RuntimeError('CLI command failed: ' + shlex.join(args))
    return p


def doc(args, **kwargs):
    return json.loads(run([*args, '--json'], **kwargs).stdout)


def check(label, condition):
    note(f'CHECK {label}: {"PASS" if condition else "FAIL"}')
    return condition


project = None
try:
    note('Journey 1: isolated authentication and config persistence')
    for attempt in (1, 2):
        directory = WORK / f'auth-{attempt}'
        auth = env | {'BOOKBEAM_CONFIG_DIR': str(directory)}
        p = run(['whoami'], use_env=auth, required=False)
        check(f'fresh config unauthenticated ({attempt})', p.returncode == 1 and 'bookbeam auth login' in p.stderr)
        result = doc(['auth', 'login', '--token', 'exploratory-fake-token'], use_env=auth)
        path = directory / 'config.json'
        check(f'login result and stored token ({attempt})', result['logged_in'] and json.loads(path.read_text())['token'] == 'exploratory-fake-token')
        note(f'fresh modes dir={stat.S_IMODE(directory.stat().st_mode):04o} config={stat.S_IMODE(path.stat().st_mode):04o}')
        snapshot = path.read_bytes()
        overridden = auth | {'BOOKBEAM_TOKEN': 'one-off-fake-token', 'BOOKBEAM_HOST': 'http://127.0.0.1:1'}
        run(['whoami'], use_env=overridden, required=False)
        check(f'overrides did not persist ({attempt})', snapshot == path.read_bytes())
        result = doc(['auth', 'logout'], use_env=auth)
        check(f'logout removed stored token ({attempt})', not result['logged_in'] and 'token' not in json.loads(path.read_text()))
        # Ordinary migration: an existing config copied/restored with default file permissions.
        path.chmod(0o644)
        run(['auth', 'login', '--token', 'exploratory-permission-sentinel'], use_env=auth)
        check(f'login secures pre-existing 0644 file ({attempt})', stat.S_IMODE(path.stat().st_mode) == 0o600)
        note(f'existing config after login: mode={stat.S_IMODE(path.stat().st_mode):04o}; sentinel persisted={json.loads(path.read_text()).get("token") == "exploratory-permission-sentinel"}')
        doc(['auth', 'logout'], use_env=auth)

    note('Journey 2: live project and signup-link lifecycle')
    profile = doc(['whoami'])
    check('identity ready', bool(profile.get('id')))
    before = doc(['projects', 'list'])
    before_ids = [p['id'] for p in before['data']]
    project = str(doc(['projects', 'create', '--title', 'Exploratory 2026-10-10 disposable', '--description', 'Initial description'])['data']['id'])
    (WORK / 'created-project-id').write_text(project)
    doc(['projects', 'update', project, '--title', 'Exploratory renamed'])
    saved = doc(['projects', 'get', project])['data']
    check('title update preserves description', saved['title'] == 'Exploratory renamed' and saved['description'] == 'Initial description')
    doc(['projects', 'update', project, '--clear-description'])
    check('description cleared after reopen', doc(['projects', 'get', project])['data']['description'] in (None, ''))
    p = run(['projects', 'update', project, '--description', 'Conflict', '--clear-description'], required=False)
    check('conflicting flags fail without changing state', p.returncode == 1 and doc(['projects', 'get', project])['data']['description'] in (None, ''))
    link = str(doc(['links', 'create', project, '--title', 'Sample chapter', '--consent', 'Join my newsletter'])['data']['id'])
    doc(['links', 'update', project, link, '--title', 'Updated chapter'])
    links = doc(['links', 'list', project])['data']
    check('link title update preserves consent', links[0]['title'] == 'Updated chapter' and links[0]['opt_in_text'] == 'Join my newsletter')
    doc(['links', 'update', project, link, '--clear-consent'])
    check('consent clear persisted', doc(['links', 'list', project])['data'][0]['opt_in_text'] in (None, ''))
    run(['links', 'delete', project, link], stdin='n\n')
    check('cancel leaves link', len(doc(['links', 'list', project])['data']) == 1)
    doc(['links', 'delete', project, link, '--force'])
    check('forced delete persisted', doc(['links', 'list', project])['data'] == [])

    note('Journey 3: book file round trip and subscriber export')
    epub = WORK / 'two words.epub'
    with zipfile.ZipFile(epub, 'w') as z:
        z.writestr('mimetype', 'application/epub+zip', compress_type=zipfile.ZIP_STORED)
        z.writestr('META-INF/container.xml', '<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>')
        z.writestr('content.opf', '<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">exploratory</dc:identifier><dc:title>Exploratory</dc:title><dc:language>en</dc:language></metadata><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/></spine></package>')
        z.writestr('chapter.xhtml', '<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Test</title></head><body><p>Disposable exploratory fixture.</p></body></html>')
    file_id = str(doc(['files', 'upload', project, epub])['data']['id'])
    files = doc(['files', 'list', project])['data']
    check('upload listed with original filename', files[0]['filename'] == epub.name)
    destination = WORK / 'downloaded.epub'
    result = doc(['files', 'download', project, file_id, '-o', destination])
    check('explicit download byte-identical', destination.read_bytes() == epub.read_bytes() and result['bytes_written'] == epub.stat().st_size)
    # Preserve upload fixture while checking automatic filename negotiation.
    expected = epub.read_bytes()
    epub.unlink()
    result = doc(['files', 'download', project, file_id])
    check('automatic filename download byte-identical', result['path'] == epub.name and epub.read_bytes() == expected)
    run(['files', 'delete', project, file_id], stdin='n\n')
    check('cancel leaves file', len(doc(['files', 'list', project])['data']) == 1)
    doc(['files', 'delete', project, file_id, '--force'])
    check('forced file delete persisted', doc(['files', 'list', project])['data'] == [])
    check('empty project has no downloaders', doc(['downloaders', 'list', project])['data'] == [])
    result = doc(['downloaders', 'export', project, '-o', WORK / 'empty.csv'])
    data = (WORK / 'empty.csv').read_bytes()
    check('empty CSV metadata matches saved file', result['bytes_written'] == len(data))
    note('empty CSV rows: ' + repr(list(csv.reader(io.StringIO(data.decode())))))
    result = doc(['downloaders', 'export', project])
    check('default CSV survives reopening and matches explicit export', (WORK / result['path']).read_bytes() == data)
    run(['projects', 'delete', project], stdin='n\n')
    check('cancel leaves project', doc(['projects', 'get', project])['data']['id'] == int(project))
except Exception:
    note(traceback.format_exc())
    raise
finally:
    if project:
        p = run(['projects', 'delete', project, '--force', '--json'], required=False)
        if p.returncode == 0:
            (WORK / 'created-project-id').unlink(missing_ok=True)
            p = run(['projects', 'get', project, '--json'], required=False)
            check('project removed (GET 404)', p.returncode == 1 and '404' in p.stderr)
            after = doc(['projects', 'list'])
            check('catalog IDs and count restored', [p['id'] for p in after['data']] == before_ids and after['total'] == before['total'])
        else:
            note('CLEANUP BLOCKED: project remains; see created-project-id')
    check('real config unchanged', SOURCE.read_bytes() == original)
    note('Real config SHA-256 unchanged: ' + hashlib.sha256(original).hexdigest())
    log.close()
