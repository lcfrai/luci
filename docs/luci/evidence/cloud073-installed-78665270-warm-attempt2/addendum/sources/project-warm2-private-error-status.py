from pathlib import Path
import json,hashlib,re,os
B=Path('/workspace/.luci-tools/cloud073-installed-78665270/evidence/warm-attempt2');P=B/'driver/private-transport-errors.jsonl';O=B/'private-error-status-projection.json'
def sha(b):return hashlib.sha256(b).hexdigest()
raw=P.read_bytes();assert len(raw)<=131072
r=json.loads((B/'driver/result.json').read_text());assert sha(raw)==r['privateTransportErrors']['sha256'] and len(raw)==r['privateTransportErrors']['bytes']
rows=[]
patterns=[('chromium-unexpected-handshake-status',re.compile(r'^Error during WebSocket handshake: Unexpected response code: ([1-5][0-9]{2})$')),('unexpected-server-response-status',re.compile(r'^Unexpected server response: ([1-5][0-9]{2})$'))]
codes=['ERR_CONNECTION_REFUSED','ERR_CONNECTION_RESET','ERR_CONNECTION_CLOSED','ERR_SOCKET_NOT_CONNECTED','ERR_WS_PROTOCOL_ERROR','ERR_FAILED','ERR_ABORTED','ERR_NAME_NOT_RESOLVED','ERR_EMPTY_RESPONSE']
for rawline in raw.splitlines():
 x=json.loads(rawline);assert set(x)=={'record','channel','ordinal','wall','monotonicMs','errorBytes','errorSHA256','error'};assert x['channel'] in ['cdp','playwright'] and type(x['ordinal']) is int and 1<=x['ordinal']<=8;assert isinstance(x['error'],str)
 b=x['error'].encode();assert len(b)<=2048 and len(b)==x['errorBytes'] and sha(b)==x['errorSHA256'];text=x['error'].strip();kind='unclassified';status=None;network=None
 for name,pattern in patterns:
  m=pattern.fullmatch(text)
  if m:kind=name;status=int(m[1]);break
 if status is None:
  found=[code for code in codes if re.search(r'(?<![A-Z0-9_])net::'+code+r'(?![A-Z0-9_])',text)]
  if len(found)==1:kind='allowlisted-network-code';network=found[0]
 rows.append({**{k:x[k] for k in ['record','channel','ordinal','wall','monotonicMs','errorBytes','errorSHA256']},'classification':kind,'httpStatus':status,'networkCode':network,'rawTextRetained':False})
assert P.read_bytes()==raw
result={'version':1,'source':str(P),'original':{'bytes':len(raw),'sha256':sha(raw)},'rows':rows,'scope':'Read-only schema, exact message length/hash checks and allowlisted source-literal/status classification. No raw text, URL, query, credentials or private log bytes exported. Error text only; no reconstructed HTTP response body, wire status, server cause or successful Restart acknowledgement implied.'}
assert not O.exists();O.write_text(json.dumps(result,indent=2)+'\n');os.chmod(O,0o600)
print(json.dumps({'projection':str(O),'sha256':sha(O.read_bytes()),'rows':len(rows),'classifications':sorted(set((x['channel'],x['classification'],x['httpStatus'],x['networkCode'],x['errorBytes'],x['errorSHA256']) for x in rows))},indent=2))
