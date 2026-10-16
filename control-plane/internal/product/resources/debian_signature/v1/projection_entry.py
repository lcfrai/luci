"""Fixed product projection child; never an ownership or eligibility verifier.

The Go owner must hold the exact resource and copied-input original guards for
this entire child lifetime. Path reads here do not establish those capabilities.
No filesystem, process, network or projection effects occur on module import.
"""
import hashlib
import json
import os
import stat
import sys
import time

PROTOCOL = 'luci-debian-signature-go-owner-v1'
REQUEST_SCHEMA = 'luci-debian-product-projection-request-v1'
CONFIG_SCHEMA = 'luci-debian-product-projection-config-v1'
OUTPUT_SCHEMA = 'luci-product-debian-signature-input-evidence-v1'
PURE_BYTES = 15522
PURE_SHA = '8401b441e3ecccd386aace046d71bb7a0941f74d105addc6a22965f7bdae0f3b'
CONFIG_BYTES = 12245
CONFIG_SHA = 'aa05c650ad8d8175838cfbe9aeab2c2ce62feec5034480cee89caa7c13f591f4'
POLICY_SHA = '13bd77f40356ef37ae01f3baf05172a035423815b33e9a292a7736ec699150a5'
LOCK_SHA = '91f2739b1ccfb8c5277e41ebbcaa45ba7ba16f1018abdf0122fc39c1ec0d0423'
ACQUISITION_SHA = 'f839f2d1edaf0fbc1f039d20fcc4abd83b04accba2d0846e369174a0f2bad8b2'
SNAPSHOT_SHA = '615c3e2f894afdad54d0ff676117ae6be513b4bf500b05660cecbcb5238f7feb'
REQUEST_MAX = 65536
OUTPUT_MAX = 262144
READ_MAX = 2182321  # 13 capped payloads + request + exact pure source + EOFs.
CALL_MAX = 2048
ROLES = (('config', 'config.json', 65536),
         ('snapshot', 'snapshot.json', 4096),
         ('key-list', 'key-list.txt', 65536)) + tuple(
    ('status-' + str(i), 'status-' + str(i) + '.txt', 131072)
    for i in range(5)) + tuple(
    ('plaintext-' + str(i), 'plaintext-' + str(i) + '.txt', 262144)
    for i in range(5))
MISSING = ['index-paragraphs', 'package-source-bindings', 'installed-manifest',
           'source-retention', 'notice-closure', 'output-proof']
ERROR_BYTES = (b'{"schema":"luci-product-projection-error-v1",'
               b'"code":"projection_refused"}\n')


class Refusal(ValueError):
    pass


class Budget:
    """Data reads only; loader IO and original-handle proof remain Go-owned."""
    def __init__(self, deadline, maximum=READ_MAX, calls=CALL_MAX, clock=time.monotonic):
        self.deadline, self.maximum, self.calls, self.clock = deadline, maximum, calls, clock
        self.used, self.count, self.failed = 0, 0, False

    def fail(self):
        self.failed = True
        raise Refusal('projection_refused')

    def check(self):
        if self.failed or self.clock() >= self.deadline:
            self.fail()

    def need(self, condition):
        self.check()
        if not condition:
            self.fail()

    def read(self, stream, requested):
        self.check()
        self.need(type(requested) is int and 0 < requested <= 16384)
        self.need(self.count < self.calls and self.used + requested <= self.maximum)
        # Admission precedes even a failing OS read. Failure keeps its charge.
        self.count += 1
        self.used += requested
        try:
            value = stream.read(requested)
        except BaseException:
            self.fail()
        self.need(type(value) is bytes and len(value) <= requested)
        self.used -= requested - max(1, len(value))
        return value


def integer(value, maximum):
    return type(value) is int and 0 <= value <= maximum


def digest_shape(value):
    return type(value) is str and len(value) == 64 and all(c in '0123456789abcdef' for c in value)


def only(value, keys, budget):
    budget.need(type(value) is dict and set(value) == set(keys))


def strict_json(data, budget):
    budget.check()
    def pairs(items):
        value = {}
        for key, item in items:
            budget.need(key not in value)
            value[key] = item
        return value
    def no_number(_):
        budget.fail()
    try:
        value = json.loads(data.decode('utf-8', 'strict'), object_pairs_hook=pairs,
                           parse_float=no_number, parse_constant=no_number)
    except BaseException:
        budget.fail()
    def bounded(node, depth=0):
        budget.need(depth <= 16)
        if type(node) is dict:
            budget.need(len(node) <= 4096)
            for key, item in node.items():
                budget.need(type(key) is str and len(key) <= 256)
                bounded(item, depth + 1)
        elif type(node) is list:
            budget.need(len(node) <= 4096)
            for item in node:
                bounded(item, depth + 1)
        elif type(node) is str:
            budget.need(len(node) <= 65536 and '\x00' not in node)
        else:
            budget.need(node is None or type(node) in (int, bool))
    bounded(value)
    return value


def read_file(path, maximum, budget, expected=None, opener=open):
    budget.check()
    budget.need(type(maximum) is int and 0 < maximum <= OUTPUT_MAX)
    if expected is not None:
        budget.need(type(expected) is tuple and len(expected) == 2)
        budget.need(integer(expected[0], maximum) and digest_shape(expected[1]))
        limit = expected[0]
    else:
        limit = maximum
    # Refuse an exhausted aggregate/call envelope before opening a file too.
    budget.need(budget.count < budget.calls and budget.used + limit + 1 <= budget.maximum)
    # The Go-held ancestor/file guards prevent namespace substitution. This
    # regular-file check and content digest alone are not original-handle proof.
    stream = None
    try:
        budget.check()
        stream = opener(path, 'rb', buffering=0)
        budget.need(stat.S_ISREG(os.fstat(stream.fileno()).st_mode))
        data = bytearray()
        while True:
            value = budget.read(stream, min(16384, limit + 1 - len(data)))
            if not value:
                break
            data.extend(value)
            budget.need(len(data) <= limit)
        budget.check()
        if expected is not None:
            budget.need(len(data) == expected[0] and hashlib.sha256(data).hexdigest() == expected[1])
        return bytes(data)
    except BaseException:
        budget.fail()
    finally:
        if stream is not None:
            try:
                stream.close()
            except BaseException:
                budget.failed = True
                raise Refusal('projection_refused') from None


def request_value(data, budget):
    budget.need(type(data) is bytes and len(data) <= REQUEST_MAX)
    value = strict_json(data, budget)
    only(value, ('schema', 'protocol', 'observedUnix', 'children', 'files'), budget)
    budget.need(value['schema'] == REQUEST_SCHEMA and value['protocol'] == PROTOCOL)
    budget.need(integer(value['observedUnix'], 253402300799))
    children = value['children']
    budget.need(type(children) is list and len(children) == 9)
    for i, child in enumerate(children):
        only(child, ('ordinal', 'exit'), budget)
        budget.need(integer(child['ordinal'], 8) and child['ordinal'] == i and
                    integer(child['exit'], 4294967295) and
                    child['exit'] in ((0,) if i < 4 else (0, 2)))
    files = value['files']
    budget.need(type(files) is list and len(files) == len(ROLES))
    for ref, (role, _, maximum) in zip(files, ROLES):
        only(ref, ('role', 'bytes', 'sha256'), budget)
        budget.need(ref['role'] == role and integer(ref['bytes'], maximum) and digest_shape(ref['sha256']))
    budget.need(files[0]['bytes'] == CONFIG_BYTES and files[0]['sha256'] == CONFIG_SHA)
    budget.need(files[1]['bytes'] == 372 and files[1]['sha256'] == SNAPSHOT_SHA)
    return value


def config_value(data, budget):
    budget.need(type(data) is bytes and len(data) == CONFIG_BYTES and
                hashlib.sha256(data).hexdigest() == CONFIG_SHA)
    value = strict_json(data, budget)
    only(value, ('schema', 'protocol', 'trustPolicySHA256', 'projectionSHA256',
                 'dependencyLockSHA256', 'acquisitionManifestSHA256',
                 'snapshotSourceRef', 'projectionConfig'), budget)
    budget.need(value['schema'] == CONFIG_SCHEMA and value['protocol'] == PROTOCOL and
                value['trustPolicySHA256'] == POLICY_SHA and value['projectionSHA256'] == PURE_SHA and
                value['dependencyLockSHA256'] == LOCK_SHA and value['acquisitionManifestSHA256'] == ACQUISITION_SHA)
    budget.need(value['snapshotSourceRef'] == {'role': 'base-rootfs-debian-sources-snapshot',
                                             'bytes': 372, 'sha256': SNAPSHOT_SHA})
    only(value['projectionConfig'], ('keys', 'releases', 'historicalAnchor'), budget)
    budget.need(len(value['projectionConfig']['keys']) == 3 and
                len(value['projectionConfig']['releases']) == 5)
    return value


def load_pure(source, budget):
    budget.need(type(source) is bytes and len(source) == PURE_BYTES and
                hashlib.sha256(source).hexdigest() == PURE_SHA)
    # The only executed source is the exact immutable application resource8401.
    # It has no lifecycle or import-time external IO; runtime stdlib closure is
    # a separate missing product-package/current-host prerequisite.
    namespace = {'__name__': 'luci_product_status_projection'}
    try:
        budget.check()
        exec(compile(source, 'luci_product_status_projection', 'exec'), namespace)
        budget.check()
    except BaseException:
        budget.fail()
    return namespace


def project(request, config, payloads, pure, budget):
    budget.check()
    semantic = config['projectionConfig']
    now = request['observedUnix']
    expected = {k['primaryFingerprint']: k['signingSubkeyFingerprint'] for k in semantic['keys']}
    try:
        records = pure['keys'](payloads['key-list'], expected, now)
        observation = dict(state='pinned-official-key-snapshot-identities-derived',
                           observedUtc=now, records=records, unseenRevocations='not-established')
        projected = []
        for i, release in enumerate(semantic['releases']):
            budget.check()
            roots = {k['primaryFingerprint'] for k in semantic['keys'] if k['id'] in release['allowedRootIDs']}
            dispositions = pure['signatures'](payloads['status-' + str(i)],
                                               request['children'][4 + i]['exit'], records, roots, now)
            plaintext = payloads['plaintext-' + str(i)]
            logical = pure['release'](plaintext, release['candidates'], now,
                                      semantic['historicalAnchor'], payloads['snapshot'])
            projected.append(dict(inputSha256=release['sha256'],
                                  signedTextSha256=hashlib.sha256(plaintext).hexdigest(),
                                  dispositions=dispositions, logicalBindings=logical))
        budget.check()
        return dict(schema=OUTPUT_SCHEMA, trustPolicySha256=POLICY_SHA,
                    keyObservation=observation, releases=projected,
                    consumer='server-captured AssemblyInputClosureRef; mandatory ValidateCurrent',
                    missing=list(MISSING), monotonicPolicyState='not-updated-by-this-read-only-batch')
    except BaseException:
        budget.fail()


def output_bytes(value, budget):
    budget.check()
    try:
        data = json.dumps(value, sort_keys=True, separators=(',', ':'),
                          ensure_ascii=True, allow_nan=False).encode('utf-8') + b'\n'
    except BaseException:
        budget.fail()
    budget.need(len(data) <= OUTPUT_MAX)
    return data


def write_output(path, data, budget, opener=open):
    budget.need(type(data) is bytes and len(data) <= OUTPUT_MAX)
    stream = None
    try:
        budget.check()
        stream = opener(path, 'xb', buffering=0)
        offset, calls = 0, 0
        while offset < len(data):
            budget.check()
            budget.need(calls < CALL_MAX)
            calls += 1
            count = stream.write(data[offset:])
            budget.need(type(count) is int and 0 < count <= len(data) - offset)
            offset += count
        budget.check()
        os.fsync(stream.fileno())
    except BaseException:
        budget.fail()
    finally:
        if stream is not None:
            try:
                stream.close()
            except BaseException:
                budget.failed = True
                raise Refusal('projection_refused') from None


def run(argv, stdin, budget):
    try:
        run_fixed(argv, stdin, budget)
    except BaseException:
        budget.fail()


def run_fixed(argv, stdin, budget):
    budget.need(type(argv) is list and len(argv) == 4)
    budget.need(all(type(arg) is str and '\x00' not in arg for arg in argv))
    directory = os.getcwd()
    pure_path, request_path, output_path = argv[1:]
    budget.need(os.path.isabs(pure_path) and os.path.basename(pure_path) == 'status_projection.py')
    budget.need(os.path.normcase(os.path.abspath(request_path)) ==
                os.path.normcase(os.path.join(directory, 'projection-request.json')) and
                os.path.normcase(os.path.abspath(output_path)) ==
                os.path.normcase(os.path.join(directory, 'producer-output')))
    budget.need(budget.read(stdin, 1) == b'')
    request = request_value(read_file(request_path, REQUEST_MAX, budget), budget)
    payloads = {}
    for ref, (role, filename, maximum) in zip(request['files'], ROLES):
        payloads[role] = read_file(os.path.join(directory, filename), maximum, budget,
                                   (ref['bytes'], ref['sha256']))
    config = config_value(payloads['config'], budget)
    pure = load_pure(read_file(pure_path, PURE_BYTES, budget, (PURE_BYTES, PURE_SHA)), budget)
    write_output(output_path, output_bytes(project(request, config, payloads, pure, budget), budget), budget)


def main():
    budget = Budget(time.monotonic() + 5)
    try:
        run(sys.argv, sys.stdin.buffer, budget)
    except BaseException:
        # One fixed, <=2KiB diagnostic channel write is permitted after refusal;
        # no further filesystem read, decoder, pure execution or output open.
        try:
            sys.stderr.buffer.write(ERROR_BYTES)
            sys.stderr.buffer.flush()
        except BaseException:
            pass
        return 2
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
