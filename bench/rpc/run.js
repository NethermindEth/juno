import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';

function requiredEnv(name) {
  const v = __ENV[name];
  if (v === undefined || v === '') {
    throw new Error(`${name} is required (pass with -e ${name}=...)`);
  }
  return v;
}

export function parseIntStrict(raw, label) {
  const n = parseInt(raw, 10);
  if (Number.isNaN(n)) {
    throw new Error(`${label} must be an integer (got "${raw}")`);
  }
  return n;
}

const NODE_URL = requiredEnv('NODE_URL');

// k6 open() cannot seek a pipe, so the corpus must be redirected in (`< file`), not piped.
const corpus = new SharedArray('corpus', () => JSON.parse(open('/dev/stdin')).requests);

// JSON-RPC errors still return HTTP 200, so success is defined by the presence of a `result`.
function isSuccess(res) {
  if (res.status !== 200) {
    return false;
  }
  let body;
  try {
    body = res.json();
  } catch (_e) {
    return false;
  }
  const hasResult = (r) => r !== null && typeof r === 'object' && 'result' in r;
  return Array.isArray(body) ? body.every(hasResult) : hasResult(body);
}

// k6 sends no Accept-Encoding by default, so gzip must be opted into (-e GZIP=1).
// k6 transparently decompresses the response before isSuccess parses it.
const headers = { 'Content-Type': 'application/json' };
if (__ENV.GZIP === '1') {
  headers['Accept-Encoding'] = 'gzip';
}

export default function measure() {
  const entry = corpus[exec.scenario.iterationInTest % corpus.length];
  const res = http.post(NODE_URL, JSON.stringify(entry), { headers });
  check(res, { 'rpc call ok': isSuccess });
}
