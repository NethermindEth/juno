import measure, { parseIntStrict } from './run.js';

const DURATION = __ENV.DURATION || '5s';
const RATES = (__ENV.RATES || '1000,2000,3000').split(',').map((r) => parseIntStrict(r.trim(), 'RATES'));
const VUS = __ENV.VUS ? parseIntStrict(__ENV.VUS, 'VUS') : Math.max(...RATES);

export const options = {
  scenarios: {
    measure: {
      executor: 'ramping-arrival-rate',
      preAllocatedVUs: VUS,
      stages: RATES.map((target) => ({ target, duration: DURATION })),
    },
  },
};

export default measure;
