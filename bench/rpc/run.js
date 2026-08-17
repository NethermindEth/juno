import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';

const vuFailures = new Counter('vu_failures');

function requiredEnv(name) {
  const v = __ENV[name];
  if (v === undefined || v === '') {
    throw new Error(`${name} is required (pass with -e ${name}=...)`);
  }
  return v;
}

export function parseIntStrict(raw, label) {
  if (!/^[0-9]+$/.test(raw)) {
    throw new Error(`${label} must be an integer (got "${raw}")`);
  }
  const n = Number(raw);
  if (!Number.isSafeInteger(n)) {
    throw new Error(`${label} must be a safe integer (got "${raw}")`);
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

export default function measure() {
  try {
    const entry = corpus[exec.scenario.iterationInTest % corpus.length];
    const res = http.post(NODE_URL, JSON.stringify(entry), {
      headers: { 'Content-Type': 'application/json' },
    });
    const requestSucceeded = isSuccess(res);
    check(requestSucceeded, { 'rpc call ok': (success) => success });
  } catch (error) {
    vuFailures.add(1);
    throw error;
  }
}
