import io, json, os, pathlib, pty, select, subprocess, tempfile, time, zipfile, threading
from http.server import HTTPServer, BaseHTTPRequestHandler
root = pathlib.Path(__file__).resolve().parents[1]
archive = io.BytesIO()
with zipfile.ZipFile(archive, 'w') as z:
    z.writestr('AndroidManifest.xml', '<manifest package="com.example.app"/>')
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200); self.end_headers(); self.wfile.write(archive.getvalue())
    def log_message(self, *args): pass
server = HTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
with tempfile.TemporaryDirectory() as directory:
    folder = pathlib.Path(directory)
    adb = folder / 'adb'
    adb.write_text('#!/bin/sh\ncase "$1" in\ndevices) printf "serial device\\n";;\n-s) printf "Success\\n";;\n*) exit 1;;\nesac\n')
    adb.chmod(0o755)
    ios = folder / 'ios'
    ios.write_text('#!/bin/sh\nprintf \'{"deviceList":[]}\\n\'\n'); ios.chmod(0o755)
    url = f'http://127.0.0.1:{server.server_port}/app.apk?token=secret'
    (folder / 'recent-urls.json').write_text(json.dumps([url, url.replace('app.apk', 'other.apk')]))
    env = dict(os.environ, FERRIE_HOME=directory, FERRIE_ADB=str(adb), FERRIE_IOS=str(ios), TERM='xterm-256color')
    env.pop('NO_COLOR', None)
    master, slave = pty.openpty()
    process = subprocess.Popen([str(root/'bin/ferrie'), '--device', 'serial'], stdin=slave, stdout=slave, stderr=slave, env=env)
    os.close(slave)
    output = bytearray()
    def read_until(fragment, seconds=10):
        stop = time.monotonic() + seconds
        while fragment not in output:
            assert time.monotonic() < stop, repr(output)
            if select.select([master], [], [], .1)[0]:
                try: output.extend(os.read(master, 65536))
                except OSError: break
        assert fragment in output, repr(output)
    read_until(b'[f] selection')
    os.write(master, b'f')
    read_until(b'\x1b[1;37;41m')
    os.write(master, b'j')
    os.write(master, b'k\r')
    read_until(b'Installing app.apk on serial')
    assert process.wait(timeout=10) == 0, repr(output)
    assert b'Confirm?' not in output
    assert b'token=secret' not in output
    assert b'\x1b[?1049l\x1b[?25h' in output
    os.close(master)
    print('PTY selection, red highlight, Enter installation, hidden query and terminal restoration passed')
server.shutdown()
