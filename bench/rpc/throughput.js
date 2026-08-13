import measure, { parseIntStrict } from './run.js';

const DURATION = __ENV.DURATION || '5s';
const RATES = (__ENV.RATES || '1000,2000,3000').split(',').map((r) => parseIntStrict(r.trim(), 'RATES'));
const THROUGHPUT_VUS = parseIntStrict(__ENV.THROUGHPUT_VUS || '50', 'THROUGHPUT_VUS');

if (RATES.some((rate) => rate <= 0)) {
  throw new Error('RATES must contain positive integers');
}
if (THROUGHPUT_VUS <= 0) {
  throw new Error('THROUGHPUT_VUS must be a positive integer');
}

export const options = {
  scenarios: {
    measure: {
      executor: 'ramping-arrival-rate',
      preAllocatedVUs: THROUGHPUT_VUS,
      maxVUs: THROUGHPUT_VUS,
      stages: RATES.map((target) => ({ target, duration: DURATION })),
    },
  },
};

export default measure;
