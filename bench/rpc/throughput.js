import { buildOptions, summarize, measure, intEnv, parseIntStrict } from './common.js';

const VUS = intEnv('VUS', 50);
const DURATION = __ENV.DURATION || '30s';
const RATES = (__ENV.RATES || '50,100,200').split(',').map((r) => parseIntStrict(r.trim(), 'RATES'));

export const options = buildOptions({
  executor: 'ramping-arrival-rate',
  startRate: 0,
  timeUnit: '1s',
  preAllocatedVUs: VUS,
  maxVUs: Math.max(VUS, ...RATES),
  stages: RATES.map((target) => ({ target, duration: DURATION })),
});

export { measure };
export const handleSummary = summarize('throughput', { VUS, RATES, DURATION });
