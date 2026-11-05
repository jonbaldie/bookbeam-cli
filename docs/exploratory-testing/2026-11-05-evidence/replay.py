#!/usr/bin/env python3
"""Exercise a built bookbeam CLI against a disposable in-process HTTP fixture."""
import http.server
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import threading

ROOT = pathlib.Path(__file__).resolve().parents[3]
BIN = ROOT / '.explore/bookbeam'
CSV = b'email,signup_link_title\n' + b'reader@example.test,Sample\n' * 12
BOOK = b'fixture-book-bytes\n'
requests = []

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_GET(self):
        requests.append((self.path, self.headers.get('Authorization')))
        if self.path == '/api/v1/user':
            data = json.dumps({'id': 1, 'name': 'Fixture Reader', 'email': 'reader@example.test', 'current_team': {'id': 1, 'name': 'Fixture Team'}}).encode()
        elif self.path == '/api/v1/projects/7/export-downloaders':
            data = CSV
        elif self.path == '/api/v1/projects/7/files/8/download':
            data = BOOK
        elif self.path == '/api/v1/projects/7/files/9/download':
            self.send_response(200)
            self.send_header('Content-Length', '100')
            self.end_headers()
            self.wfile.write(b'partial')
            self.close_connection = True
            return
        else:
            self.send_error(404)
            return
        self.send_response(200)
        if self.path.endswith('/download'):
            self.send_header('Content-Disposition', 'attachment; filename="fixture.epub"')
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def run(args, env, cwd, limited=False):
    argv = [str(BIN), *args]
    if limited:
        wrapper = ('import os, resource, signal, sys; '
                   'signal.signal(signal.SIGXFSZ, signal.SIG_IGN); '
                   'resource.setrlimit(resource.RLIMIT_FSIZE, (64, 64)); '
                   'os.execv(sys.argv[1], sys.argv[1:])')
        argv = [sys.executable, '-I', '-c', wrapper, *argv]
    p = subprocess.run(argv, env=env, cwd=cwd, capture_output=True, text=True, timeout=10)
    print('$ bookbeam', ' '.join(args))
    print('exit=', p.returncode, 'stdout=', repr(p.stdout), 'stderr=', repr(p.stderr))
    return p


def main():
    server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    print('fixture=', server.server_address, 'owner=main thread; teardown=shutdown/server_close/join')
    try:
        with tempfile.TemporaryDirectory(dir=ROOT / '.explore') as temp:
            d = pathlib.Path(temp)
            env = os.environ.copy()
            env.pop('BOOKBEAM_TOKEN', None)
            env['BOOKBEAM_CONFIG_DIR'] = str(d / 'config')
            env['BOOKBEAM_HOST'] = f'http://127.0.0.1:{server.server_port}'
            print('\nAUTH')
            assert run(['auth', 'login', '--token', 'fake-token', '--json'], env, d).returncode == 0
            config = d / 'config/config.json'
            print('config mode=', oct(config.stat().st_mode & 0o777), 'token saved=', json.loads(config.read_text())['token'] == 'fake-token')
            assert config.stat().st_mode & 0o777 == 0o600 and json.loads(config.read_text())['token'] == 'fake-token'
            assert json.loads(run(['whoami', '--json'], env, d).stdout)['name'] == 'Fixture Reader'
            assert requests[-1][1] == 'Bearer fake-token'
            assert run(['auth', 'logout', '--json'], env, d).returncode == 0
            print('logout removed token=', not json.loads(config.read_text()).get('token'))
            assert not json.loads(config.read_text()).get('token')
            n = len(requests)
            assert run(['whoami'], env, d).returncode != 0
            print('after logout requests sent=', len(requests) - n)
            assert len(requests) == n
            print('\nCSV EXPORT')
            env['BOOKBEAM_TOKEN'] = 'fake-token'
            p = run(['downloaders', 'export', '7', '--output', 'subscribers.csv', '--json'], env, d)
            assert p.returncode == 0 and (d / 'subscribers.csv').read_bytes() == CSV
            print('disk CSV matches fixture=', True, 'mode=', oct((d / 'subscribers.csv').stat().st_mode & 0o777))
            p = run(['downloaders', 'export', '7', '--output', '-'], env, d)
            print('stdout CSV matches fixture=', p.stdout.encode() == CSV)
            assert p.stdout.encode() == CSV
            for i in range(2):
                target = d / f'keep{i}.csv'
                original = b'previous complete export\n'
                target.write_bytes(original)
                p = run(['downloaders', 'export', '7', '-o', target.name], env, d, limited=True)
                print('limited replay=', i + 1, 'previous preserved=', target.read_bytes() == original, 'length=', target.stat().st_size, 'partial matches new=', target.read_bytes() == CSV[:64])
                assert p.returncode != 0 and target.read_bytes() == CSV[:64]
            print('\nFILE DOWNLOAD')
            p = run(['files', 'download', '7', '8', '--json'], env, d)
            assert p.returncode == 0 and (d / 'fixture.epub').read_bytes() == BOOK
            print('default download matches fixture=', True)
            (d / 'keep.epub').write_bytes(b'complete old book')
            for i in range(2):
                p = run(['files', 'download', '7', '9', '-o', 'keep.epub'], env, d)
                print('short body replay=', i + 1, 'old book preserved=', (d / 'keep.epub').read_bytes() == b'complete old book', 'staged files=', len(list(d.glob('.bookbeam-download-*'))))
                assert p.returncode != 0 and (d / 'keep.epub').read_bytes() == b'complete old book'
                assert not list(d.glob('.bookbeam-download-*'))
            print('requests=', requests)
    finally:
        server.shutdown()
        server.server_close()
        thread.join()
        print('fixture stopped=', not thread.is_alive())

if __name__ == '__main__':
    main()
