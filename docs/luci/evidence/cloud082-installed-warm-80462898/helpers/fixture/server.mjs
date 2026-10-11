import { createServer } from 'node:http';
import { readFileSync } from 'node:fs';
import { calculate } from './calculator.mjs';
const proof = JSON.parse(readFileSync(new URL('./build-proof.json', import.meta.url), 'utf8'));
const page = '<!doctype html><html lang="en"><meta name="viewport" content="width=device-width"><title>Preview calculator</title><h1>Preview calculator</h1><form action="/sum"><label>First amount <input name="a" inputmode="decimal" value="12.5" required></label><label>Second amount <input name="b" inputmode="decimal" value="7.25" required></label><button>Add amounts</button></form></html>';
createServer((request, response) => {
  const url = new URL(request.url, 'http://fixture.invalid');
  response.setHeader('Cache-Control', 'no-store');
  if (request.method !== 'GET') { response.writeHead(405); response.end(); return; }
  if (url.pathname === '/') { response.setHeader('Content-Type', 'text/html; charset=utf-8'); response.end(page); return; }
  response.setHeader('Content-Type', 'application/json');
  if (url.pathname === '/build-proof') { response.end(JSON.stringify(proof)); return; }
  if (url.pathname !== '/sum') { response.writeHead(404); response.end(JSON.stringify({error: 'Not found'})); return; }
  try {
    const sum = calculate(url.searchParams.get('a'), url.searchParams.get('b'));
    response.end(JSON.stringify({greeting: process.env.APP_GREETING ?? '', modelKeyPresent: ['OPENAI_API_KEY', 'ANTHROPIC_API_KEY', 'GEMINI_API_KEY'].some(key => Boolean(process.env[key])), sum, uid: process.getuid()}));
  } catch { response.writeHead(400); response.end(JSON.stringify({error: 'Enter two finite decimal amounts'})); }
}).listen(3000, '0.0.0.0');
