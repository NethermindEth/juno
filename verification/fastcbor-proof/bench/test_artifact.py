import unittest
import check


class ArtifactSchedule(unittest.TestCase):
    def fixture(self):
        return {'status': 'pass', 'looks': [{'rounds': 5}],
                'samples': {f'{r}:case': {} for r in range(1, 6)}}

    def test_incomplete_live_report_cannot_pass(self):
        report = self.fixture()
        report['status'] = 'running'
        with self.assertRaisesRegex(ValueError, 'incomplete'):
            check.check_schedule(report, {'case': 'compatible_fast'})

    def test_missing_pair_rejected(self):
        report = self.fixture()
        del report['samples']['5:case']
        with self.assertRaisesRegex(ValueError, 'missing or extra'):
            check.check_schedule(report, {'case': 'compatible_fast'})

    def test_extra_pair_rejected(self):
        report = self.fixture()
        report['samples']['6:case'] = {}
        with self.assertRaisesRegex(ValueError, 'missing or extra'):
            check.check_schedule(report, {'case': 'compatible_fast'})

    def test_skipped_look_rejected(self):
        report = self.fixture()
        report['looks'] = [{'rounds': 10}]
        with self.assertRaisesRegex(ValueError, 'prespecified prefix'):
            check.check_schedule(report, {'case': 'compatible_fast'})

    def test_decision_comparison_uses_ids_not_json_key_order(self):
        left = {'rounds': 5, 'cases': [{'id': 'b', 'status': 'pass'}, {'id': 'a', 'status': 'fail'}]}
        right = {'rounds': 5, 'cases': list(reversed(left['cases']))}
        self.assertEqual(check.canonical_look(left), check.canonical_look(right))
        right['cases'][0] = {'id': 'a', 'status': 'pass'}
        self.assertNotEqual(check.canonical_look(left), check.canonical_look(right))

    def test_duplicate_decision_id_rejected(self):
        with self.assertRaisesRegex(ValueError, 'duplicate case'):
            check.canonical_look({'cases': [{'id': 'a'}, {'id': 'a'}]})

    def test_controls_required_exclusions_preserved(self):
        report = self.fixture()
        classes = {'case': 'compatible_fast', 'control': 'zero_decoder_control', 'removed': 'incompatible_removed_projection'}
        with self.assertRaisesRegex(ValueError, 'missing or extra'):
            check.check_schedule(report, classes)
        report['samples'].update({f'{r}:control': {} for r in range(1, 6)})
        self.assertEqual(check.check_schedule(report, classes), report['looks'])


if __name__ == '__main__':
    unittest.main()
