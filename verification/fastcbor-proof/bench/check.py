#!/usr/bin/env python3
"""Validate a terminal gate artifact and recompute its decisions from raw samples.

This checks provenance and internal consistency, not proof of environmental
isolation. It rejects partial runs and never promotes inconclusive to passing.
"""
import argparse
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import zipfile

import run


def require(condition, message):
    if not condition:
        raise ValueError(message)


def check_schedule(report, classes):
    require(report['status'] in ('pass', 'fail', 'inconclusive'), 'run is incomplete; no terminal gate result')
    looks = report['looks']
    require(bool(looks), 'terminal run has no planned look')
    rounds = [look['rounds'] for look in looks]
    require(rounds == list(run.LOOKS[:len(rounds)]), 'looks are not the prespecified prefix')
    expected_keys = {f'{r}:{case_id}' for r in range(1, rounds[-1] + 1) for case_id, state in classes.items() if state != 'incompatible_removed_projection'}
    require(set(report['samples']) == expected_keys, 'missing or extra paired samples')
    return looks


def canonical_look(look):
    ids = [case['id'] for case in look['cases']]
    require(len(ids) == len(set(ids)), 'duplicate case in saved decision')
    return {**look, 'cases': sorted(look['cases'], key=lambda case: case['id'])}


def check_report(directory, report, verify_binaries=True):
    identity = report['identity']
    looks = check_schedule(report, identity['classifications'])
    universe = json.loads(run.UNIVERSE.read_text())
    require(identity['runner'] == run.sha(run.__file__), 'runner changed')
    require(identity['harness'] == run.sha(run.HERE / 'frozen_bench_test.go.txt'), 'harness changed')
    require(identity['universe'] == run.sha(run.UNIVERSE), 'frozen universe changed')
    require(identity['protocol'] == universe['protocol'], 'protocol changed')
    require(identity['alpha'] == run.ALPHA and identity['scipy'] == run.scipy.__version__, 'statistical inputs changed')
    require(identity['environment'] == {
        'GOTOOLCHAIN': 'go1.27.0', 'GOWORK': 'off', 'GOOS': 'linux', 'GOARCH': 'amd64',
        'GOAMD64': 'v1', 'GOFLAGS': '', 'GOMAXPROCS': '1', 'GOGC': '100', 'GOMEMLIMIT': 'off',
    }, 'benchmark environment changed')
    for label, filename, evidence in [
        ('ordinary', 'compatibility-smoke.json', 'compatibility_smoke'),
        ('race/checkptr', 'compatibility-smoke-race-checkptr.json', 'race_checkptr_smoke'),
    ]:
        path = run.PROOF / 'results' / filename
        require(identity[evidence] == run.sha(path), f'{label} compatibility evidence changed')
        smoke = json.loads(path.read_text())
        for target in ('baseline', 'candidate'):
            require(report[target]['sources'] == smoke[target]['tracked_go_source_sha256'], f'{label} {target} source mismatch')
            require(identity['classifications'] == {row['id']: row['status'] for row in smoke[target]['cases']}, f'{label} case classification mismatch')
    classes = identity['classifications']
    require(set(classes) == {case['id'] for case in universe['cases']}, 'case universe changed')
    require(len(classes) == 185, 'expected all 185 frozen cases')
    require(sum(x == 'compatible_fast' for x in classes.values()) == 177, 'compatible case count changed')
    require(sum(x == 'incompatible_removed_projection' for x in classes.values()) == 6, 'excluded case count changed')
    require(sum(x == 'zero_decoder_control' for x in classes.values()) == 2, 'control case count changed')
    for label, repo in [('baseline', run.BASELINE), ('candidate', run.REPO)]:
        saved = report[label]
        require(saved['sources'] == run.sources(repo), f'{label} sources changed')
        if verify_binaries:
            require(saved['binary'] == run.sha(directory / label / 'core.test'), f'{label} binary changed')
        require(saved['wrapper'] == run.sha(directory / label / 'wrapper.go'), f'{label} wrapper changed')
        require(saved['harness'] == run.sha(directory / label / 'frozen_bench_test.go'), f'{label} copied harness changed')
        require(saved['harness'] == identity['harness'], f'{label} harness identity mismatch')
        expected = subprocess.check_output(['gofmt'], input=run.fallback_overlay((repo / 'db/typed/value/cbor.go').read_text()), text=True)
        require((directory / label / 'wrapper.go').read_text() == expected, f'{label} overlay differs from source-derived fallback restriction')
        preflight = (directory / label / 'preflight.log').read_text()
        for test in ('TestFrozenGateOperations', 'TestFrozenGateNoFallback'):
            require(f'--- PASS: {test} ' in preflight, f'{label} missing preflight {test}')
        require(preflight.rstrip().endswith('PASS'), f'{label} preflight not successful')
    require(report['baseline']['revision'] == universe['baseline_commit'], 'baseline commit changed')
    require(run.git(run.BASELINE, 'rev-parse', 'HEAD') == universe['baseline_commit'], 'baseline checkout changed')
    require(not run.git(run.BASELINE, 'status', '--porcelain'), 'baseline is dirty')
    raw_names = set()
    for key, pair in report['samples'].items():
        _, case_id = key.split(':', 1)
        require(set(pair) == {'baseline', 'candidate'}, 'incomplete paired sample')
        for label, sample in pair.items():
            relative = Path(sample['raw'])
            require(not relative.is_absolute() and '..' not in relative.parts and relative.parts[0] == 'raw', 'invalid raw path')
            require(str(relative) not in raw_names, 'raw sample reused')
            raw_names.add(str(relative))
            path = directory / relative
            require(run.sha(path) == sample['sha256'], f'raw sample changed: {relative}')
            text = path.read_text()
            require(text.rstrip().endswith('PASS'), f'benchmark did not finish successfully: {relative}')
            rows = [line.split() for line in text.splitlines() if line.startswith('BenchmarkFrozenGate/')]
            require(len(rows) == 1, f'expected exactly one benchmark row: {relative}')
            row = rows[0]
            require(row[0] == 'BenchmarkFrozenGate/' + case_id, 'raw benchmark identity differs')
            require(int(row[1]) == sample['iterations'] and sample['iterations'] > 0, 'raw iteration count differs')
            metrics = {row[i+1]: float(row[i]) for i in range(2, len(row)-1, 2)}
            require(all(metrics[m] == sample[m] for m in run.METRICS), 'saved metrics differ from raw output')
    for index, look in enumerate(looks):
        require(canonical_look(look) == canonical_look(run.analyze(report['samples'], classes, look['rounds'])), 'saved decision differs from recomputation')
        if index < len(looks)-1:
            require(look['status'] == 'inconclusive', 'samples collected after a terminal planned look')
    require(report['status'] == looks[-1]['status'], 'terminal status disagrees with last look')
    if report['status'] == 'inconclusive':
        require(looks[-1]['rounds'] == 20, 'inconclusive run stopped before final planned look')
    return {'status': report['status'], 'rounds': looks[-1]['rounds'], 'pairs': len(report['samples']), 'raw_files': len(raw_names), 'decoder_cases': looks[-1]['decoder_cases'], 'binary_check': 'verified files' if verify_binaries else 'hashes recorded; binaries excluded from archive'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path, help='run directory, or evidence ZIP with --archive')
    parser.add_argument('--archive', action='store_true', help='verify saved logs/sources/decisions; executable hashes are recorded but binaries are not archived')
    args = parser.parse_args()
    for flags in ([], ['--race', '--checkptr']):
        subprocess.run([sys.executable, str(run.PROOF / 'smoke/run.py'), *flags, '--check'], check=True)
    if args.archive:
        with tempfile.TemporaryDirectory(prefix='juno-cbor-evidence-') as temporary:
            directory = Path(temporary)
            with zipfile.ZipFile(args.directory) as archive:
                require(len(archive.namelist()) == len(set(archive.namelist())), 'duplicate archive paths')
                require(sum(info.file_size for info in archive.infolist()) < 100_000_000, 'unexpected archive size')
                for info in archive.infolist():
                    relative = Path(info.filename)
                    require(not relative.is_absolute() and '..' not in relative.parts, 'invalid archive path')
                    require(info.filename == 'report.json' or len(relative.parts) == 2 and relative.parts[0] in ('raw', 'baseline', 'candidate'), 'unexpected archive member')
                    target = directory / relative
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(archive.read(info))
            report = json.loads((directory / 'report.json').read_text())
            print(json.dumps(check_report(directory, report, verify_binaries=False), indent=2))
    else:
        report = json.loads((args.directory / 'report.json').read_text())
        print(json.dumps(check_report(args.directory, report), indent=2))


if __name__ == '__main__':
    main()
