"""Bounded GET /info through Docker's already selected CLI transport.

`docker system dial-stdio` uses dockerCli.Client().Dialer(): the same context,
endpoint and TLS/SSH transport authority as ordinary Docker commands. No host,
context, environment, credentials or daemon settings are replaced here.
Unlike `docker info`, it does not enumerate CLI plugin metadata processes.
"""
import http.client
import io
import os
import re
import subprocess
import threading
import time

import archive

MAX_BODY = 1 << 20
MAX_HEADERS = 64 << 10
MAX_STDERR = 64 << 10
REQUEST = b'GET /info HTTP/1.1\r\nHost: docker\r\nAccept: application/json\r\nConnection: close\r\n\r\n'


class _HeldBytes(io.BytesIO):
    def close(self):
        pass  # Keep the bounded cursor inspectable after HTTPResponse closes.


class _Socket:
    def __init__(self, raw):
        self.stream = _HeldBytes(raw)
    def makefile(self, *args):
        return self.stream


def decode(raw, expected_engine=None):
    header_end = raw.find(b'\r\n\r\n')
    if not 0 <= header_end <= MAX_HEADERS or len(raw) > MAX_BODY + MAX_HEADERS:
        raise ValueError('selected engine info response framing exceeds bound')
    if not re.fullmatch(rb'HTTP/1\.[01] 200(?: [^\r\n]*)?\r\n[\s\S]*', raw[:header_end + 2]):
        raise ValueError('selected engine info requires a successful direct HTTP response')
    socket = _Socket(raw)
    response = http.client.HTTPResponse(socket)
    try:
        response.begin()
        if response.status != 200 or response.getheader('Location') is not None:
            raise ValueError('selected engine info redirected or failed')
        names = [name.lower() for name, _ in response.getheaders()]
        if len(names) > 64 or any(names.count(name) > 1 for name in ('content-length', 'transfer-encoding', 'content-type', 'content-encoding', 'location')):
            raise ValueError('selected engine info duplicate or excessive headers')
        length, transfer = response.getheader('Content-Length'), response.getheader('Transfer-Encoding')
        if (length is not None and (not re.fullmatch('[0-9]+', length) or not 1 <= int(length) <= MAX_BODY) or
                transfer is not None and transfer.lower() != 'chunked' or length is not None and transfer is not None or
                response.getheader('Content-Encoding') not in (None, 'identity') or
                (response.getheader('Content-Type') or '').split(';', 1)[0].strip().lower() != 'application/json'):
            raise ValueError('selected engine info unsupported HTTP framing/type')
        body = response.read(MAX_BODY + 1)
        if (not 1 <= len(body) <= MAX_BODY or length is not None and len(body) != int(length) or
                response.read(1) or socket.stream.tell() != len(raw)):
            raise ValueError('selected engine info body exceeds bound or has trailing bytes')
        value = archive.json_bytes(body)
    except (http.client.HTTPException, OSError) as error:
        raise ValueError('selected engine info HTTP response invalid') from error
    finally:
        response.close()
    if (not isinstance(value, dict) or not isinstance(value.get('ID'), str) or not re.fullmatch(r'[\x21-\x7e]{1,160}', value['ID']) or
            value.get('OSType') != 'linux' or not isinstance(value.get('OperatingSystem'), str) or
            not 1 <= len(value['OperatingSystem']) <= 256):
        raise ValueError('selected Linux engine identity unavailable')
    if expected_engine is not None and value['ID'] != expected_engine:
        raise ValueError('selected engine identity changed')
    return value


def read(expected_engine=None, timeout=5):
    if type(timeout) not in (int, float) or not 0 < timeout <= 15:
        raise ValueError('selected engine info deadline invalid')
    if expected_engine is not None and (not isinstance(expected_engine, str) or not re.fullmatch(r'[\x21-\x7e]{1,160}', expected_engine)):
        raise ValueError('expected engine identity invalid')
    process = subprocess.Popen(['docker', 'system', 'dial-stdio'], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
        stderr=subprocess.PIPE, creationflags=0x08000000 if os.name == 'nt' else 0)
    buffers = [bytearray(), bytearray()]
    failures, stop = [], threading.Event()
    def drain(stream, buffer, limit):
        try:
            with stream:
                while True:
                    block = stream.read(16 << 10)
                    if not block:
                        break
                    if len(buffer) + len(block) > limit:
                        raise ValueError('selected engine info transport output exceeds bound')
                    buffer.extend(block)
        except BaseException as error:
            failures.append(error)
            stop.set()
    readers = [threading.Thread(target=drain, args=(stream, buffer, limit), daemon=True)
               for stream, buffer, limit in ((process.stdout, buffers[0], MAX_BODY + MAX_HEADERS), (process.stderr, buffers[1], MAX_STDERR))]
    for reader in readers:
        reader.start()
    error = None
    try:
        process.stdin.write(REQUEST)
        process.stdin.close()
        deadline = time.monotonic() + timeout
        while process.poll() is None:
            if failures:
                raise ValueError('selected engine info transport read failed')
            if time.monotonic() >= deadline:
                raise ValueError('selected engine info transport deadline exceeded')
            stop.wait(.01)
    except BaseException as caught:
        error = caught
    finally:
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=2)
        for reader in readers:
            reader.join(timeout=2)
        if any(reader.is_alive() for reader in readers):
            error = error or ValueError('selected engine info readers failed to retire')
    if error is not None:
        raise error
    if failures or process.returncode != 0:
        raise ValueError('selected engine info transport failed; no fallback')
    return decode(bytes(buffers[0]), expected_engine)
