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

export function intEnv(name, def) {
  return parseIntStrict(__ENV[name] || String(def), name);
}

export const NODE_URL = requiredEnv('NODE_URL');

// k6 open() cannot seek a pipe, so the corpus must be redirected in (`< file`), not piped.
let parsed;
const load = () => (parsed ??= JSON.parse(open('/dev/stdin')));
const corpus = new SharedArray('corpus', () => load().requests);
const meta = new SharedArray('meta', () => [load().meta]);

export function buildOptions(measured) {
  return {
    scenarios: { measure: { exec: 'measure', ...measured } },
    summaryTrendStats: ['avg', 'min', 'med', 'p(50)', 'p(90)', 'p(99)', 'max'],
  };
}

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

export function measure() {
  const entry = corpus[exec.scenario.iterationInTest % corpus.length];
  const res = http.post(NODE_URL, JSON.stringify(entry), {
    headers: { 'Content-Type': 'application/json' },
  });
  check(res, { 'rpc call ok': isSuccess });
}

function stat(data, metric, key) {
  const m = data.metrics[metric];
  if (!m || !m.values || m.values[key] === undefined) {
    return null;
  }
  return m.values[key];
}

function fmt(n) {
  return n === null ? 'n/a' : n.toFixed(2);
}

function buildMarkdown(summary) {
  const m = summary.metrics;
  const knobs = Object.entries(summary.knobs)
    .map(([k, v]) => `${k}=${Array.isArray(v) ? v.join(',') : v}`)
    .join(' ');
  return [
    `# RPC benchmark — ${summary.method}`,
    '',
    `- **Node:** ${summary.node}`,
    `- **Scenario:** ${summary.scenario}`,
    `- **Knobs:** ${knobs}`,
    '',
    '| Metric | Value |',
    '| --- | --- |',
    `| p50 latency (ms) | ${fmt(m.latency_ms.p50)} |`,
    `| p90 latency (ms) | ${fmt(m.latency_ms.p90)} |`,
    `| p99 latency (ms) | ${fmt(m.latency_ms.p99)} |`,
    `| avg latency (ms) | ${fmt(m.latency_ms.avg)} |`,
    `| max latency (ms) | ${fmt(m.latency_ms.max)} |`,
    `| throughput (rpc/s) | ${fmt(m.throughput_rps)} |`,
    `| requests (measured) | ${fmt(m.requests)} |`,
    `| dropped iterations | ${fmt(m.dropped_iterations)} |`,
    `| pass rate | ${fmt(m.pass_rate)} |`,
    '',
  ].join('\n');
}

export function summarize(scenario, knobs) {
  return (data) => {
    const durationMetric = 'http_req_duration';
    const httpReqRate = stat(data, 'http_reqs', 'rate');
    const rpcPerReq = Math.max(1, meta[0].batch);
    const summary = {
      node: NODE_URL,
      method: meta[0].method,
      scenario,
      knobs,
      metrics: {
        latency_ms: {
          p50: stat(data, durationMetric, 'p(50)'),
          p90: stat(data, durationMetric, 'p(90)'),
          p99: stat(data, durationMetric, 'p(99)'),
          avg: stat(data, durationMetric, 'avg'),
          max: stat(data, durationMetric, 'max'),
        },
        throughput_rps: httpReqRate === null ? null : httpReqRate * rpcPerReq,
        requests: stat(data, 'http_reqs', 'count'),
        dropped_iterations: stat(data, 'dropped_iterations', 'count'),
        pass_rate: stat(data, 'checks', 'rate'),
      },
    };
    const markdown = buildMarkdown(summary);
    return {
      stdout: `${JSON.stringify(summary, null, 2)}\n`,
      stderr: `\n${markdown}\n`,
    };
  };
}
