export const gtmMetricNames = ['visitors', 'signups', 'activations', 'purchases', 'gross_revenue', 'refunds', 'spend'] as const;
export type GTMMetricName = typeof gtmMetricNames[number];
export interface GTMMetricRow { row: number; metric: GTMMetricName; start: string; end: string; value: number; currency: string; decimals: number }
export interface GTMMetricImport { version: 1; sourceName: string; sourceSha256: string; sourceBytes: number; rows: GTMMetricRow[] }
export const metricCSVTemplate = 'metric,start,end,value,currency,decimals\nvisitors,2026-01-01,2026-01-31,1000,,\nsignups,2026-01-01,2026-01-31,50,,\ngross_revenue,2026-01-01,2026-01-31,1250.00,USD,2\n';
const money = (metric: string) => ['gross_revenue', 'refunds', 'spend'].includes(metric);
const validDay = (day: string) => /^\d{4}-\d{2}-\d{2}$/.test(day) && Number.isFinite(Date.parse(day + 'T00:00:00Z')) && new Date(day + 'T00:00:00Z').toISOString().slice(0, 10) === day;
export function metricImportsValid(value: unknown): value is GTMMetricImport[] {
 if (!Array.isArray(value) || value.length > 8) return false;
 const sources = new Set<string>(); let total = 0;
 for (const b of value as GTMMetricImport[]) {
  if (!b || b.version !== 1 || typeof b.sourceName !== 'string' || b.sourceName !== b.sourceName.trim() || !b.sourceName || new TextEncoder().encode(b.sourceName).length > 120 || /[\x00-\x1f\x7f/\\]/.test(b.sourceName) || typeof b.sourceSha256 !== 'string' || !/^[a-f0-9]{64}$/.test(b.sourceSha256) || sources.has(b.sourceSha256) || !Number.isSafeInteger(b.sourceBytes) || b.sourceBytes < 1 || b.sourceBytes > 128 * 1024 || !Array.isArray(b.rows) || b.rows.length < 1 || b.rows.length > 128) return false;
  sources.add(b.sourceSha256); total += b.rows.length; if (total > 128) return false;
  const keys = new Set<string>(); let prior = 1;
  for (const r of b.rows) {
   if (!r || !Number.isSafeInteger(r.row) || r.row <= prior || r.row > 129 || !gtmMetricNames.includes(r.metric) || typeof r.start !== 'string' || typeof r.end !== 'string' || !validDay(r.start) || !validDay(r.end) || Date.parse(r.end) < Date.parse(r.start) || Date.parse(r.end) - Date.parse(r.start) > 3660 * 86400000 || !Number.isSafeInteger(r.value) || r.value < 0 || r.value > 1e12 || !Number.isInteger(r.decimals)) return false;
   prior = r.row;
   if (money(r.metric) ? typeof r.currency !== 'string' || !/^[A-Z]{3}$/.test(r.currency) || r.decimals < 0 || r.decimals > 6 : r.currency !== '' || r.decimals !== 0 || r.value > 1e9) return false;
   const key = [r.metric, r.start, r.end, r.currency].join('|'); if (keys.has(key)) return false; keys.add(key);
  }
 }
 return true;
}
function cells(line: string): string[] {
 const out: string[] = []; let value = '', quoted = false, closed = false;
 for (let i = 0; i < line.length; i++) {
  const c = line[i];
  if (quoted) { if (c === '"') { if (line[i + 1] === '"') { value += '"'; i++; } else { quoted = false; closed = true; } } else value += c; }
  else if (c === ',') { out.push(value); value = ''; closed = false; }
  else if (closed) throw new Error('Unexpected characters after a quoted CSV field.');
  else if (c === '"') { if (value) throw new Error('A CSV quote must begin a field.'); quoted = true; }
  else value += c;
 }
 if (quoted) throw new Error('Use one aggregate record per line; multiline fields are not accepted.');
 out.push(value); return out;
}
export async function parseMetricCSV(bytes: ArrayBuffer, sourceName: string): Promise<GTMMetricImport> {
 if (bytes.byteLength < 1 || bytes.byteLength > 128 * 1024) throw new Error('Choose a CSV between 1 byte and 128 KiB.');
 const raw = new TextDecoder('utf-8', { fatal: true }).decode(bytes).replace(/\r\n/g, '\n');
 if (raw.includes('\r')) throw new Error('Use ordinary CSV line endings.');
 const lines = raw.split('\n'); if (lines[lines.length - 1] === '') lines.pop();
 if (lines.length < 2 || lines.length > 129 || JSON.stringify(cells(lines[0])) !== JSON.stringify(['metric','start','end','value','currency','decimals'])) throw new Error('Use the template headers and 1–128 aggregate rows.');
 const rows: GTMMetricRow[] = lines.slice(1).map((line, index) => {
  const fields = cells(line); if (fields.length !== 6) throw new Error('Each row needs exactly the six template fields.');
  const [metric, start, end, rawValue, currency, rawDecimals] = fields;
  const isMoney = money(metric);
  if (isMoney ? !/^[0-6]$/.test(rawDecimals) : rawDecimals !== '' || currency !== '') throw new Error('Money needs a currency and decimal count; counts leave those fields empty.');
  const decimals = isMoney ? Number(rawDecimals) : 0;
  if (!/^(0|[1-9]\d{0,12})(?:\.\d{1,6})?$/.test(rawValue) || !isMoney && rawValue.includes('.')) throw new Error('Use nonnegative counts or plain decimal money, without formulas or separators.');
  const [whole, fraction = ''] = rawValue.split('.'); if (fraction.length > decimals) throw new Error('An amount has more decimal places than declared.');
  const minor = BigInt(whole) * 10n ** BigInt(decimals) + BigInt(fraction.padEnd(decimals, '0') || '0');
  if (minor > 1_000_000_000_000n) throw new Error('An amount exceeds the import limit.');
  return { row: index + 2, metric: metric as GTMMetricName, start, end, value: Number(minor), currency, decimals };
 });
 const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), x => x.toString(16).padStart(2, '0')).join('');
 const result: GTMMetricImport = { version: 1, sourceName, sourceSha256: digest, sourceBytes: bytes.byteLength, rows };
 if (!metricImportsValid([result])) throw new Error('Check metric names, dates, currency, duplicate rows and filename. Use only aggregate results.');
 return result;
}
export function metricValue(row: GTMMetricRow): string {
 if (!money(row.metric)) return String(row.value);
 const digits = String(row.value).padStart(row.decimals + 1, '0');
 return row.currency + ' ' + (row.decimals ? digits.slice(0, -row.decimals) + '.' + digits.slice(-row.decimals) : digits);
}
