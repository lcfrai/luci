"""Decode one bounded proof archive in memory. Never extract or write a file."""
import io,json,sys,tarfile

try:
    raw=sys.stdin.buffer.read(65537)
    if not raw or len(raw)>65536 or len(raw)%512:
        raise ValueError('archive size')
    with tarfile.open(fileobj=io.BytesIO(raw),mode='r:') as archive:
        members=archive.getmembers()
        if len(members)!=1:
            raise ValueError('member count')
        member=members[0]
        if member.name!='stage-proof.json' or not member.isreg() or member.linkname or not (0<member.size<=16384):
            raise ValueError('member identity, type or size')
        # Allow only bounded timestamp metadata from daemon TAR writers.
        # Path/link/size overrides and arbitrary extended attributes refuse.
        if set(member.pax_headers)-{'mtime','atime','ctime'} or any(len(str(v))>64 for v in member.pax_headers.values()):
            raise ValueError('extended metadata')
        end=member.offset_data+((member.size+511)//512)*512
        if end+1024>len(raw) or any(raw[end:]):
            raise ValueError('archive end marker or trailing data')
        stream=archive.extractfile(member)
        payload=stream.read(16385)
        if len(payload)!=member.size or len(payload)>16384:
            raise ValueError('payload size')
        value=json.loads(payload.decode('utf-8',errors='strict'))
        if not isinstance(value,dict):
            raise ValueError('proof shape')
        sys.stdout.write(json.dumps(value,separators=(',',':')))
except Exception:
    # No source-supplied name, metadata, JSON or daemon error is printed.
    sys.stderr.write('Exact bounded proof archive refused\n')
    raise SystemExit(1)
