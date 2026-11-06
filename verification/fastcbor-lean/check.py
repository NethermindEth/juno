#!/usr/bin/env python3
"""Repeat feature assessment; expected translation failures are test results, not proofs."""
from pathlib import Path
import hashlib
import json
import os
import subprocess

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parent.parent
TOOLS = ROOT / '.tools'
RESULTS = ROOT / 'results'
ENV = os.environ | {'GOTOOLCHAIN': os.environ.get('GOTOOLCHAIN', 'go1.27.0'), 'GOWORK': 'off'}
RESULTS.mkdir(exist_ok=True)
records = []


def run(name, args, *, cwd=ROOT, expected=0, contains=()):
    proc = subprocess.run([str(a) for a in args], cwd=cwd, env=ENV, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=900)
    output = proc.stdout.replace(str(REPO), '<JUNO>')
    (RESULTS / (name + '.log')).write_text(output)
    records.append({'case': name, 'exit': proc.returncode, 'expected_exit': expected})
    print(f'{name}: exit {proc.returncode} (expected {expected})', flush=True)
    assert proc.returncode == expected, f'{name}: see results/{name}.log'
    for snippet in contains:
        assert snippet in output, f'{name}: missing diagnostic {snippet!r}'
    return output


def translate(name, tool, config, out, module, patterns, **kwargs):
    return run(name, [TOOLS / 'bin' / tool, '-dir', REPO if module == 'juno' else ROOT / 'probes',
                     '-configdir', ROOT / config, '-out', ROOT / out, '-lean-root',
                     ('github.com/NethermindEth/juno' if module == 'juno' else 'fastcborprobe')
                     + '=FastcborCheck', *patterns], **kwargs)


revision = (ROOT / 'juno.rev').read_text().strip()
perennial = subprocess.check_output(['git', '-C', str(TOOLS / 'perennial'), 'rev-parse', 'HEAD'], text=True).strip()
assert perennial == (ROOT / 'perennial.rev').read_text().strip()
# Check actual source against the pinned branch, even after assessment commits.
source_hashes = {}
for directory in ['utils/cbor/fastcbor', 'utils/cbor/internal/cursor']:
    for path in sorted((REPO / directory).glob('*.go')):
        if path.name.endswith('_test.go'):
            continue
        relative = path.relative_to(REPO).as_posix()
        pinned = subprocess.check_output(['git', 'show', revision + ':' + relative], cwd=REPO)
        assert path.read_bytes() == pinned, f'source changed: {relative}'
        source_hashes[relative] = hashlib.sha256(pinned).hexdigest()
(RESULTS / 'source.json').write_text(json.dumps({'juno': revision, 'perennial': perennial,
                                               'sha256': source_hashes}, indent=2) + '\n')

run('go-tests', ['go', 'test', '-short', './utils/cbor/internal/cursor',
                 './utils/cbor/fastcbor', './utils/cbor/compatibility'], cwd=REPO)
run('probe-go-build', ['go', 'test', '-short', './...'], cwd=ROOT / 'probes')
packages = ['./utils/cbor/internal/cursor', './utils/cbor/fastcbor']
translate('full-goose', 'goose', 'config', '.tools/full', 'juno', packages,
          expected=1, contains=['unsafe.SliceData', 'external function', 'unsupported type uintptr'])
translate('filtered-goose', 'goose', 'filtered-config', '.tools/filtered', 'juno', packages)
translate('filtered-proofgen', 'proofgen', 'filtered-config', '.tools/filtered-proof', 'juno', packages)

failures = {'external': 'external function', 'slicedata': 'unsafe.SliceData',
            'uintptrinc': 'unsupported type uintptr', 'genericsize': 'unsafe.Sizeof',
            'genericarray': 'unexpected type for indexing'}
passing = []
for case in sorted((ROOT / 'probes').iterdir()):
    if not case.is_dir():
        continue
    failed = case.name in failures
    translate('probe-' + case.name, 'goose', 'config', 'FastcborCheck/Code', 'probe',
              ['./' + case.name], expected=int(failed),
              contains=[failures[case.name]] if failed else [])
    if not failed:
        passing.append('./' + case.name)
translate('proofgen', 'proofgen', 'config', 'FastcborCheck/GeneratedProof', 'probe', passing)
for tool in ['goose', 'proofgen']:
    translate('core-' + tool, tool, 'core-config',
              'FastcborCheck/' + ('Code' if tool == 'goose' else 'GeneratedProof'),
              'juno', ['./utils/cbor/internal/cursor'])
# Check our generated support code, not every imported upstream module.
for path in (ROOT / 'FastcborCheck/GeneratedProof').rglob('*.lean'):
    assert 'sorry' not in path.read_text(), str(path)
    assert '\naxiom ' not in path.read_text(), str(path)

elan = Path(ENV.get('ELAN_HOME', str(TOOLS / 'elan')))
ENV['ELAN_HOME'] = str(elan)
run('lean-build', [elan / 'bin/lake', 'build', 'FastcborCheck'])
# Re-elaborate so #print axioms runs even with cached build artifacts.
audit = run('boundary-axioms', [elan / 'bin/lake', 'env', 'lean', 'FastcborCheck/MemoryBoundary.lean'])
assert 'sorryAx' not in audit
assert audit.count('depends on axioms: [propext, Classical.choice, Quot.sound]') == 2
(RESULTS / 'checks.json').write_text(json.dumps(records, indent=2) + '\n')
print('Assessment reproduced. Full-library translation is expected to FAIL. No decoder correctness theorem was proved.')
