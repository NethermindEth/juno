import measure, { parseIntStrict } from './run.js';

const DURATION = __ENV.DURATION || '5s';
const RATES = (__ENV.RATES || '1000,2000,3000').split(',').map((r) => parseIntStrict(r.trim(), 'RATES'));

export const options = {
  scenarios: {
    measure: {
      executor: 'ramping-arrival-rate',
      preAllocatedVUs: Math.max(...RATES),
      stages: RATES.map((target) => ({ target, duration: DURATION })),
    },
  },
};

export default measure;
