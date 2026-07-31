import { buildOptions, summarize, measure, intEnv } from './common.js';

const ITERATIONS = intEnv('ITERATIONS', 200);

export const options = buildOptions({
  executor: 'shared-iterations',
  vus: 1,
  iterations: ITERATIONS,
});

export { measure };
export const handleSummary = summarize('single', { ITERATIONS });
