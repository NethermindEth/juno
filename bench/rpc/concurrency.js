import { buildOptions, summarize, measure, intEnv } from './common.js';

const VUS = intEnv('VUS', 50);
const DURATION = __ENV.DURATION || '30s';

export const options = buildOptions({
  executor: 'constant-vus',
  vus: VUS,
  duration: DURATION,
});

export { measure };
export const handleSummary = summarize('concurrency', { VUS, DURATION });
