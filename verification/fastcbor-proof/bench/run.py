#!/usr/bin/env python3
"""Build and measure the frozen CBOR gate using source-hashed temporary overlays.

Only the wrapper's fallback branches change (to panic); successful fast paths,
including their existing error checks, stay intact. The timing harness has no
smoke counters. Baseline files are never modified. Resume requires identical
inputs, binaries, protocol and completed paired samples.
"""
import argparse
import fcntl
import hashlib
import json
import math
import os
from pathlib import Path
import re
import statistics
import subprocess
import sys

import scipy
from scipy.stats import t

HERE = Path(__file__).resolve().parent
PROOF = HERE.parent
REPO = PROOF.parent.parent
BASELINE = REPO.parent / 'cbor-fastcbor-baseline'
UNIVERSE = PROOF / 'results/benchmark-universe.json'
ALPHA = 0.05
LOOKS = (5, 10, 20)
METRICS = ('ns/op', 'B/op', 'allocs/op')


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def git(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args], text=True).strip()


def write_json(path, value):
    temporary = path.with_suffix(path.suffix + '.tmp')
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True, allow_nan=False) + '\n')
    temporary.replace(path)


def sources(repo):
    names = subprocess.check_output(['git', '-C', str(repo), 'ls-files', '-z', '*.go', 'go.mod', 'go.sum']).decode().split('\0')
    return {name: sha(repo / name) for name in sorted(names) if name}


def fallback_overlay(source):
    # Preserve the complete success path. A fallback is an invalid measurement.
    for name in ('Unmarshal', 'UnmarshalFirst'):
        pattern = r'(func \(fastCborSerializer\[V\]\) ' + name + r'\(.*?\n\})'
        matches = list(re.finditer(pattern, source, re.S))
        if len(matches) != 1:
            raise RuntimeError('wrapper signature changed; review timing overlay')
        original = matches[0].group()
        revised, count = re.subn(r'\twarnFastCbor\[V\]\(err\)\n\treturn cbor\.' + name + r'\(data, value\)',
                                '\tpanic("CBOR benchmark attempted FX fallback")', original)
        if count != 1:
            raise RuntimeError('fallback branch changed; review timing overlay')
        source = source.replace(original, revised, 1)
    return source


def metric_decision(pairs, metric):
    base = [pair['baseline'][metric] for pair in pairs]
    candidate = [pair['candidate'][metric] for pair in pairs]
    if any(not math.isfinite(x) or x < 0 for x in base + candidate):
        raise ValueError('invalid benchmark metric')
    if metric == 'ns/op' and any(x == 0 for x in base + candidate):
        raise ValueError('nonpositive timing')
    if any(x == 0 for x in base):
        if all(x == 0 for x in base + candidate):
            return {'status': 'pass', 'reason': 'zero allocations preserved', 'ratio': 1.0}
        if all(x == 0 for x in base) and any(x > 0 for x in candidate):
            return {'status': 'fail', 'reason': 'zero allocation baseline increased'}
        return {'status': 'inconclusive', 'reason': 'baseline allocation count varies across zero'}
    # Deterministically reduced-to-zero allocation metrics need no log transform.
    if all(x == 0 for x in candidate):
        return {'status': 'pass', 'reason': 'candidate allocation metric zero', 'ratio': 0.0}
    if any(x == 0 for x in candidate):
        return {'status': 'inconclusive', 'reason': 'candidate allocation count varies across zero'}
    ratios = [math.log(c / b) for b, c in zip(base, candidate)]
    mean = statistics.mean(ratios)
    margin = float(t.ppf(1 - ALPHA / len(LOOKS), len(pairs) - 1)) * statistics.stdev(ratios) / math.sqrt(len(pairs))
    upper, lower = math.exp(mean + margin), math.exp(mean - margin)
    return {'status': 'pass' if upper <= 1.03 else 'fail' if lower > 1.03 else 'inconclusive',
            'ratio': math.exp(mean), 'lower': lower, 'upper': upper}


def analyze(samples, classifications, rounds):
    if rounds not in LOOKS:
        raise ValueError('decisions are only permitted at the prespecified looks')
    rows = []
    for case_id, classification in classifications.items():
        if classification == 'incompatible_removed_projection':
            rows.append({'id': case_id, 'status': classification})
            continue
        pairs = [samples[f'{r}:{case_id}'] for r in range(1, rounds + 1)]
        metrics = {metric: metric_decision(pairs, metric) for metric in METRICS}
        status = 'fail' if any(m['status'] == 'fail' for m in metrics.values()) else 'pass' if all(m['status'] == 'pass' for m in metrics.values()) else 'inconclusive'
        rows.append({'id': case_id, 'classification': classification, 'status': status, 'metrics': metrics})
    # The controls are measured and reported, but do not establish decoder gains.
    comparable = [row for row in rows if row.get('classification') == 'compatible_fast']
    counts = {state: sum(row['status'] == state for row in comparable) for state in ('pass', 'fail', 'inconclusive')}
    status = 'fail' if counts['fail'] else 'pass' if counts['pass'] == len(comparable) else 'inconclusive'
    return {'rounds': rounds, 'status': status, 'decoder_cases': counts, 'cases': rows}


def build(repo, label, run, env):
    directory = run / label
    directory.mkdir()
    wrapper = directory / 'wrapper.go'
    wrapper.write_text(fallback_overlay((repo / 'db/typed/value/cbor.go').read_text()))
    harness = directory / 'frozen_bench_test.go'
    harness.write_bytes((HERE / 'frozen_bench_test.go.txt').read_bytes())
    subprocess.run(['gofmt', '-w', str(wrapper), str(harness)], check=True)
    overlay = directory / 'overlay.json'
    write_json(overlay, {'Replace': {str(repo / 'db/typed/value/cbor.go'): str(wrapper), str(repo / 'core/zz_frozen_gate_test.go'): str(harness)}})
    binary = directory / 'core.test'
    with (directory / 'build.log').open('w') as log:
        subprocess.run(['go', 'test', '-c', '-overlay=' + str(overlay), '-o', str(binary), './core'], cwd=repo, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
    with (directory / 'preflight.log').open('w') as log:
        subprocess.run([str(binary), '-test.run=^TestFrozenGate', '-test.v'], cwd=repo, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
    return {'revision': git(repo, 'rev-parse', 'HEAD'), 'sources': sources(repo),
            'binary': sha(binary), 'wrapper': sha(wrapper), 'harness': sha(harness)}


def measure(run, label, case_id, round_number, env):
    raw = run / 'raw' / f'{round_number:02d}-{hashlib.sha256(case_id.encode()).hexdigest()[:16]}-{label}.txt'
    command = ['taskset', '-c', '2', str(run / label / 'core.test'), '-test.run=^$', '-test.bench=^BenchmarkFrozenGate$', '-test.benchtime=300ms', '-test.benchmem', '-test.count=1', '-test.timeout=10m']
    with raw.open('w') as out:
        subprocess.run(command, cwd=REPO, env=dict(env, JUNO_CBOR_GATE_CASE=case_id), stdout=out, stderr=subprocess.STDOUT, check=True)
    rows = [line for line in raw.read_text().splitlines() if line.startswith('BenchmarkFrozenGate/')]
    if len(rows) != 1:
        raise RuntimeError(f'expected exactly one measurement: {raw}')
    parts = rows[0].split()
    if parts[0] != 'BenchmarkFrozenGate/' + case_id or int(parts[1]) <= 0:
        raise RuntimeError(f'benchmark identity/count mismatch: {raw}')
    metrics = {parts[i + 1]: float(parts[i]) for i in range(2, len(parts) - 1, 2)}
    if not set(METRICS) <= metrics.keys():
        raise RuntimeError(f'missing metrics: {raw}')
    return {**{metric: metrics[metric] for metric in METRICS}, 'iterations': int(parts[1]), 'raw': str(raw.relative_to(run)), 'sha256': sha(raw)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('run', type=Path, help='new output directory, or existing directory with --resume')
    parser.add_argument('--resume', action='store_true')
    parser.add_argument('--prepare-only', action='store_true', help='build and preflight; collect no timings')
    args = parser.parse_args()
    run = args.run.resolve()
    subprocess.run([sys.executable, str(PROOF / 'smoke/run.py'), '--check'], check=True)
    subprocess.run([sys.executable, str(PROOF / 'smoke/run.py'), '--race', '--checkptr', '--check'], check=True)
    universe = json.loads(UNIVERSE.read_text())
    protocol = universe['protocol']
    assert protocol['paired_interleaved_rounds'] == list(LOOKS) and protocol['cpu'] == 2
    assert protocol['GOMAXPROCS'] == 1 and protocol['benchtime'] == '300ms' and protocol['max_regression'] == .03
    if 2 not in os.sched_getaffinity(0):
        raise RuntimeError('CPU 2 unavailable')
    smoke = json.loads((PROOF / 'results/compatibility-smoke.json').read_text())
    classifications = {case['id']: case['status'] for case in smoke['candidate']['cases']}
    assert classifications == {case['id']: case['status'] for case in smoke['baseline']['cases']}
    assert set(classifications.values()) == {'compatible_fast', 'incompatible_removed_projection', 'zero_decoder_control'}
    assert len(classifications) == 185
    env = dict(os.environ, GOTOOLCHAIN='go1.27.0', GOWORK='off', GOOS='linux', GOARCH='amd64', GOAMD64='v1', GOFLAGS='', GOMAXPROCS='1', GOGC='100', GOMEMLIMIT='off', JUNO_CBOR_GATE_UNIVERSE=str(UNIVERSE))
    env.pop('GODEBUG', None)
    identity = {'protocol': protocol, 'alpha': ALPHA, 'statistics': 'paired log-ratio Student t, one-sided alpha/3; all comparable cases and all metrics must pass; no cross-case correction is needed for this intersection-union acceptance rule',
                'scipy': scipy.__version__, 'runner': sha(__file__), 'harness': sha(HERE / 'frozen_bench_test.go.txt'), 'universe': sha(UNIVERSE),
                'compatibility_smoke': sha(PROOF / 'results/compatibility-smoke.json'), 'race_checkptr_smoke': sha(PROOF / 'results/compatibility-smoke-race-checkptr.json'),
                'go': subprocess.check_output(['go', 'version'], env=env, text=True).strip(), 'classifications': classifications,
                'environment': {k: env[k] for k in ('GOTOOLCHAIN','GOWORK','GOOS','GOARCH','GOAMD64','GOFLAGS','GOMAXPROCS','GOGC','GOMEMLIMIT')},
                'cpu': Path('/proc/cpuinfo').read_text().split('model name',1)[1].splitlines()[0].strip()}
    if args.resume:
        report = json.loads((run / 'report.json').read_text())
        if report['identity'] != identity:
            raise RuntimeError('run identity changed; preserve old evidence and start a new run')
        for label, repo in [('baseline', BASELINE), ('candidate', REPO)]:
            saved = report[label]
            if saved['sources'] != sources(repo) or saved['binary'] != sha(run / label / 'core.test'):
                raise RuntimeError('sources/binary changed; cannot resume')
        for pair in report['samples'].values():
            for sample in pair.values():
                if sha(run / sample['raw']) != sample['sha256']:
                    raise RuntimeError('raw sample hash changed')
        if report['status'] in ('pass','fail','inconclusive'):
            print('Run already terminal:', report['status']); return
    else:
        run.mkdir(parents=True, exist_ok=False)
        (run / 'raw').mkdir()
        report = {'identity': identity, 'samples': {}, 'looks': [], 'status': 'preparing'}
        for label, repo in [('baseline', BASELINE), ('candidate', REPO)]:
            print('Building and preflighting', label, flush=True)
            before = sources(repo)
            report[label] = build(repo, label, run, env)
            if before != sources(repo) or identity['harness'] != sha(HERE / 'frozen_bench_test.go.txt') or identity['runner'] != sha(__file__):
                raise RuntimeError('source tree changed during build')
        report['status'] = 'prepared'
        write_json(run / 'report.json', report)
    if args.prepare_only:
        print('Prepared both binaries; no timing samples collected.', flush=True); return
    print('Waiting for shared measurement lock', flush=True)
    with open(protocol['lock'], 'a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        print('Acquired shared measurement lock', flush=True)
        report['status'] = 'running'
        write_json(run / 'report.json', report)
        ids = sorted(case_id for case_id, state in classifications.items() if state != 'incompatible_removed_projection')
        for round_number in range(1, LOOKS[-1] + 1):
            for index, case_id in enumerate(ids):
                key = f'{round_number}:{case_id}'
                if key in report['samples']:
                    continue
                order = ('baseline','candidate') if (round_number + index) % 2 else ('candidate','baseline')
                pair = {label: measure(run, label, case_id, round_number, env) for label in order}
                report['samples'][key] = pair
                write_json(run / 'report.json', report)
                if index % 20 == 0:
                    print(f'Round {round_number}: {index+1}/{len(ids)} pairs', flush=True)
            for label, repo in [('baseline', BASELINE), ('candidate', REPO)]:
                if report[label]['sources'] != sources(repo):
                    raise RuntimeError('sources changed during measurement; results invalid')
            if round_number in LOOKS and not any(look['rounds'] == round_number for look in report['looks']):
                result = analyze(report['samples'], classifications, round_number)
                report['looks'].append(result)
                print('Look',round_number,result['status'],result['decoder_cases'],flush=True)
                if result['status'] in ('pass','fail') or round_number == LOOKS[-1]:
                    report['status'] = result['status']
                    write_json(run / 'report.json', report)
                    return
            write_json(run / 'report.json', report)


if __name__ == '__main__':
    main()
