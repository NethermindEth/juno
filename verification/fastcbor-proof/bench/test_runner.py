"""Checks for gate decisions which could otherwise silently accept regressions."""
import unittest
import run


def pairs(baseline, candidate, metric='ns/op'):
    return [{'baseline': {metric: b}, 'candidate': {metric: c}} for b, c in zip(baseline, candidate)]


class GateDecisions(unittest.TestCase):
    def test_accepts_improvement_and_rejects_regression(self):
        for count in run.LOOKS:
            self.assertEqual(run.metric_decision(pairs([100] * count, [90] * count), 'ns/op')['status'], 'pass')
            self.assertEqual(run.metric_decision(pairs([100] * count, [104] * count), 'ns/op')['status'], 'fail')

    def test_noise_is_not_a_pass(self):
        result = run.metric_decision(pairs([100] * 5, [80, 90, 100, 110, 120]), 'ns/op')
        self.assertEqual(result['status'], 'inconclusive')
        self.assertGreater(result['upper'], 1.03)
        self.assertLess(result['lower'], 1.03)

    def test_pairs_remove_common_drift(self):
        baseline = [100, 200, 300, 400, 500]
        result = run.metric_decision(pairs(baseline, [x * .99 for x in baseline]), 'ns/op')
        self.assertEqual(result['status'], 'pass')
        self.assertAlmostEqual(result['ratio'], .99)

    def test_zero_allocations(self):
        for metric in ('B/op', 'allocs/op'):
            self.assertEqual(run.metric_decision(pairs([0] * 5, [0] * 5, metric), metric)['status'], 'pass')
            self.assertEqual(run.metric_decision(pairs([0] * 5, [0, 0, 0, 0, 1], metric), metric)['status'], 'fail')
            self.assertEqual(run.metric_decision(pairs([1] * 5, [0] * 5, metric), metric)['status'], 'pass')
            self.assertEqual(run.metric_decision(pairs([0, 1, 0, 1, 0], [0] * 5, metric), metric)['status'], 'inconclusive')

    def test_missing_case_and_unplanned_look_rejected(self):
        with self.assertRaises(ValueError):
            run.analyze({}, {}, 6)
        with self.assertRaises(KeyError):
            run.analyze({}, {'required': 'compatible_fast'}, 5)

    def test_no_averaging_away_failed_case(self):
        classification = {'good': 'compatible_fast', 'bad': 'compatible_fast', 'removed': 'incompatible_removed_projection', 'control': 'zero_decoder_control'}
        samples = {}
        for n in range(1, 6):
            for name in ('good', 'bad', 'control'):
                samples[f'{n}:{name}'] = {'baseline': {m: 100 for m in run.METRICS}, 'candidate': {m: 104 if name == 'bad' else 80 for m in run.METRICS}}
        result = run.analyze(samples, classification, 5)
        self.assertEqual(result['status'], 'fail')
        self.assertEqual(result['decoder_cases'], {'pass': 1, 'fail': 1, 'inconclusive': 0})
        self.assertEqual(len(result['cases']), 4)


if __name__ == '__main__':
    unittest.main()
