"""Regression checks for premise reporting across Lean proof styles."""
import re
import unittest

from audit_signatures import theorem_signatures


class TheoremSignaturesTest(unittest.TestCase):
    def test_all_proof_styles_preserve_assumed_contract(self):
        for body in ['by\n  exact helper', 'rfl', 'helper', '\n  helper']:
            with self.subTest(body=body):
                source = 'theorem sample (validator : Utf8ValidatorContract (GF := GF)) : Result := ' + body
                self.assertEqual(list(theorem_signatures(source)), [
                    ('sample', ' (validator : Utf8ValidatorContract (GF := GF)) : Result ')
                ])

    def test_named_arguments_and_nested_delimiters_are_not_assignments(self):
        signature = ' {GF : Type} (h : P (GF := GF)) : {{ pre }} call {{ out }} '
        self.assertEqual(list(theorem_signatures('theorem sample' + signature + ':= helper')),
                         [('sample', signature)])

    def test_comments_and_escaped_strings_do_not_end_signature(self):
        signature = ' (x : T) /- := /- nested ) -/ -/ -- := }\n : P "escaped \\\" := )" '
        self.assertEqual(list(theorem_signatures('theorem sample' + signature + ':= by trivial')),
                         [('sample', signature)])

    def test_result_contract_is_not_an_assumed_binder(self):
        source = ('theorem implication (word : Utf8WordWindowContract (GF := GF)) :\n'
                  '    Utf8ValidatorContract (GF := GF) := implicationProof\n'
                  'theorem assumption (validator : Utf8ValidatorContract (GF := GF)) : P := by trivial\n')
        signatures = dict(theorem_signatures(source))
        assumed = {name for name, signature in signatures.items()
                   if re.search(r'\(\s*\w+\s*:\s*Utf8ValidatorContract\b', signature)}
        self.assertEqual(assumed, {'assumption'})

    def test_invalid_or_unterminated_signatures_fail_closed(self):
        for source in ['theorem bad : T', 'theorem bad (x : T] : T := by trivial',
                       'theorem bad : P "unfinished :=', 'theorem bad /- unfinished :=']:
            with self.subTest(source=source), self.assertRaises(ValueError):
                list(theorem_signatures(source))


if __name__ == '__main__':
    unittest.main()
