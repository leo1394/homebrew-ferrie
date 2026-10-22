import os, pathlib, pty, select, subprocess, tempfile, time, zipfile

root = pathlib.Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory() as directory:
    folder = pathlib.Path(directory)
    ipa = folder / 'app.ipa'
    with zipfile.ZipFile(ipa, 'w') as archive:
        archive.writestr('Payload/App.app/Info.plist', '<plist><dict><key>CFBundleIdentifier</key><string>test.app</string></dict></plist>')
    ios = folder / 'ios'
    ios.write_text('''#!/bin/sh
if [ "$1" = list ]; then
  printf '%s\\n' '{"deviceList":["test-phone"]}'
  exit 0
fi
printf '%s\\n' '{"level":"WARN","msg":"go-ios agent is not running"}' >&2
printf '%s\\n' '{"level":"INFO","msg":"installing","percentComplete":60}' >&2
sleep 0.2
printf '%s\\n' '{"level":"INFO","msg":"installing","percentComplete":100}' >&2
''')
    ios.chmod(0o755)
    xcrun = folder / 'xcrun'
    xcrun.write_text('#!/bin/sh\nexit 1\n'); xcrun.chmod(0o755)
    env = dict(os.environ, FERRIE_IOS=str(ios), FERRIE_HOME=directory, TERM='xterm-256color', PATH=directory+os.pathsep+os.environ.get('PATH', ''))
    master, slave = pty.openpty()
    process = subprocess.Popen([str(root/'bin/ferrie'), '--target', str(ipa), '--device', 'test-phone'], stdin=slave, stdout=slave, stderr=slave, env=env)
    os.close(slave)
    output = bytearray()
    stop = time.monotonic() + 15
    while time.monotonic() < stop:
        if select.select([master], [], [], .1)[0]:
            try:
                data = os.read(master, 65536)
                if not data: break
                output.extend(data)
            except OSError: break
        elif process.poll() is not None: break
    assert process.wait(timeout=2) == 0, repr(output)
    os.close(master)
    assert b'60.0%' in output and b'100.0%' in output and b'###' in output, repr(output)
    assert b'"level"' not in output and b'agent is not running' not in output, repr(output)
    assert b'Installed successfully.' in output, repr(output)
    print('PTY installation percentages, clean output and successful completion passed')
