#!/usr/bin/env python3
"""Classify every frozen benchmark case using temporary Go overlays, without timings.

No baseline or candidate production file is edited. Wrapper overlays remove FX
fallback, and FX overlays reject/count calls made inside the fast decode scope.
The test-only counters are unsuitable for collecting performance measurements.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
REPO = HERE.parent.parent.parent
BASELINE = Path('/home/daniel/.herdr/worktrees/juno/cbor-fastcbor-baseline')
BASELINE_SHA = 'a4a67242cc67f3855915a97bf4e79a97d217fae4'
UNIVERSE = HERE.parent / 'results/benchmark-universe.json'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def git(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args], text=True).strip()


def wrapper_overlay(source):
    patterns = [
        (r'func \(fastCborSerializer\[V\]\) Unmarshal\(data \[\]byte, value \*V\) error \{.*?\n\}', '''func (fastCborSerializer[V]) Unmarshal(data []byte, value *V) error {
 if !proofSmokeFast { return cbor.Unmarshal(data, value) }
 proofSmokeCalls++
 return fastcbor.Unmarshal(data, value)
}'''),
        (r'func \(fastCborSerializer\[V\]\) UnmarshalFirst\(data \[\]byte, value \*V\) \(\[\]byte, error\) \{.*?\n\}', '''func (fastCborSerializer[V]) UnmarshalFirst(data []byte, value *V) ([]byte, error) {
 if !proofSmokeFast { return cbor.UnmarshalFirst(data, value) }
 proofSmokeCalls++
 return fastcbor.UnmarshalFirst(data, value)
}'''),
    ]
    for pattern, replacement in patterns:
        source, count = re.subn(pattern, replacement, source, flags=re.S)
        if count != 1:
            raise RuntimeError('wrapper source changed; review overlay before continuing')
    source = source.replace('\n\t"errors"', '')
    return source + '''
// Test-only single-threaded instrumentation. Not a timing implementation.
var proofSmokeFast bool
var proofSmokeCalls int
func CBORProofSmokeMode(fast bool) { proofSmokeFast = fast; proofSmokeCalls = 0 }
func CBORProofSmokeCalls() int { return proofSmokeCalls }
'''


def fx_overlay(source):
    source = source.replace('import (', 'import (\n "fmt"', 1)
    for signature, result in [
        ('func Unmarshal(b []byte, v any) error {', 'return fmt.Errorf("FX call inside fast decode")'),
        ('func UnmarshalFirst(b []byte, v any) ([]byte, error) {', 'return nil, fmt.Errorf("FX first call inside fast decode")'),
        ('func UnmarshalStrict(b []byte, v any) error {', 'return fmt.Errorf("FX strict call inside fast decode")'),
    ]:
        if source.count(signature) != 1:
            raise RuntimeError('FX adapter source changed; review overlay before continuing')
        source = source.replace(signature, signature + '\n if CBORProofSmokeForbid { CBORProofSmokeCalls++; ' + result + ' }', 1)
    return source + '\nvar CBORProofSmokeForbid bool\nvar CBORProofSmokeCalls int\n'


def tracked_sources(repo):
    paths = subprocess.check_output(['git', '-C', str(repo), 'ls-files', '-z', '*.go', 'go.mod', 'go.sum']).decode().split('\0')
    return {p: sha((repo / p).read_bytes()) for p in sorted(paths) if p}


def run_target(repo, label, expected_ids, build_flags):
    revision = git(repo, 'rev-parse', 'HEAD')
    status = git(repo, 'status', '--porcelain')
    if label == 'baseline' and (revision != BASELINE_SHA or status):
        raise RuntimeError('baseline must be clean at the frozen revision')
    for source in (repo / 'core').glob('*.go'):
        if not source.name.endswith('_test.go') and re.search(r'\btype\s+headerEventsBloomProjection\b', source.read_text()):
            raise RuntimeError('historical bloom projection exists; port its cases instead of classifying it as removed')
    sources = tracked_sources(repo)
    with tempfile.TemporaryDirectory(prefix='juno-cbor-smoke-') as tmp:
        tmp = Path(tmp)
        overlay = {}
        replacements = {
            'core/zz_frozen_cbor_smoke_test.go': (HERE / 'frozen_test.go.txt').read_text(),
            'db/typed/value/cbor.go': wrapper_overlay((repo / 'db/typed/value/cbor.go').read_text()),
            'utils/cbor/fxamacker/cbor.go': fx_overlay((repo / 'utils/cbor/fxamacker/cbor.go').read_text()),
        }
        for i, (original, text) in enumerate(replacements.items()):
            replacement = tmp / f'source-{i}.go'
            replacement.write_text(text)
            subprocess.run(['gofmt', '-w', str(replacement)], check=True)
            overlay[str(repo / original)] = str(replacement)
        overlay_hashes = {str(Path(original).relative_to(repo)): sha(Path(replacement).read_bytes()) for original, replacement in overlay.items()}
        overlay_file = tmp / 'overlay.json'
        overlay_file.write_text(json.dumps({'Replace': overlay}))
        rows_path = tmp / 'rows.json'
        env = dict(os.environ, GOTOOLCHAIN='go1.27.0', GOWORK='off', GOOS='linux', GOARCH='amd64', GOAMD64='v1', GOFLAGS='',
                   JUNO_PROOF_SMOKE_UNIVERSE=str(UNIVERSE), JUNO_PROOF_SMOKE_OUTPUT=str(rows_path))
        command = ['go', 'test', *build_flags, '-overlay=' + str(overlay_file), '-run', '^TestFrozenCBORSmoke($|Instrumentation$)', '-count=1', '-timeout=10m', '-v', './core']
        log = tmp / 'go-test.log'
        with log.open('w') as out:
            result = subprocess.run(command, cwd=repo, env=env, stdout=out, stderr=subprocess.STDOUT)
        if result.returncode:
            raise RuntimeError(f'{label} smoke harness failed:\n{log.read_text()[-12000:]}')
        if '--- PASS: TestFrozenCBORSmokeInstrumentation' not in log.read_text():
            raise RuntimeError('negative instrumentation check did not pass')
        rows = json.loads(rows_path.read_text())
        if sorted(row['id'] for row in rows) != expected_ids:
            raise RuntimeError('smoke omitted or duplicated a frozen case')
        if sources != tracked_sources(repo) or status != git(repo, 'status', '--porcelain'):
            raise RuntimeError('worktree changed during smoke; results require review')
        counts = {}
        for row in rows:
            counts[row['status']] = counts.get(row['status'], 0) + 1
        print(label, revision[:12], counts, flush=True)
        return {'revision': revision, 'tracked_go_source_sha256': sources,
                'overlay_sha256': overlay_hashes, 'instrumentation_negative_test': 'passed', 'counts': counts, 'cases': rows}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='verify saved evidence source/corpus hashes without rerunning smoke')
    parser.add_argument('--race', action='store_true', help='run the Go race detector')
    parser.add_argument('--checkptr', action='store_true', help='run strict checkptr=2 instrumentation')
    parser.add_argument('--baseline', type=Path, default=BASELINE)
    parser.add_argument('--candidate', type=Path, default=REPO)
    args = parser.parse_args()
    universe = json.loads(UNIVERSE.read_text())
    if universe['baseline_commit'] != BASELINE_SHA:
        raise RuntimeError('unexpected frozen baseline')
    for path, digest in universe['sources'].items():
        if sha(Path(path).read_bytes()) != digest:
            raise RuntimeError(f'frozen historical source hash changed: {path}')
    for record in universe['records']:
        if sha(Path(record['File']).read_bytes()) != record['SHA256']:
            raise RuntimeError(f'frozen corpus hash changed: {record["File"]}')
    ids = sorted(case['id'] for case in universe['cases'])
    if len(ids) != 185 or len(set(ids)) != 185:
        raise RuntimeError('unexpected frozen universe')
    build_flags = (['-race'] if args.race else []) + (['-gcflags=all=-d=checkptr=2'] if args.checkptr else [])
    mode = ('-race' if args.race else '') + ('-checkptr' if args.checkptr else '')
    output = HERE.parent / f'results/compatibility-smoke{mode}.json'
    if args.check:
        report = json.loads(output.read_text())
        if report['go_build_flags'] != build_flags:
            raise RuntimeError('saved evidence uses different race/checkptr flags')
        if report['universe_sha256'] != sha(UNIVERSE.read_bytes()):
            raise RuntimeError('smoke evidence uses a different frozen universe')
        for path in [Path(__file__), HERE / 'frozen_test.go.txt']:
            if report['harness_sha256'][path.name] != sha(path.read_bytes()):
                raise RuntimeError('smoke harness changed; rerun smoke')
        for label, repo in [('baseline', args.baseline.resolve()), ('candidate', args.candidate.resolve())]:
            saved = report[label]
            if saved['tracked_go_source_sha256'] != tracked_sources(repo):
                raise RuntimeError(f'{label} Go sources changed; rerun smoke')
            if label == 'baseline' and (git(repo, 'rev-parse', 'HEAD') != BASELINE_SHA or git(repo, 'status', '--porcelain')):
                raise RuntimeError('frozen baseline is no longer clean at its pin')
            if sorted(row['id'] for row in saved['cases']) != ids:
                raise RuntimeError('saved smoke evidence does not cover the complete universe')
        print('Saved smoke evidence matches both source trees, harness and all frozen sources/corpus files; no performance claim.')
        return
    report = {
        'go_target': 'go1.27.0 linux/amd64/v1',
        'go_build_flags': build_flags,
        'ambient_GOFLAGS': 'cleared; only recorded flags apply',
        'status': 'compatibility classification only; no performance samples or gate result',
        'scope': 'Actual fast public APIs and block serializers with test-only wrapper overlays that remove fallback; FX calls within the fast decode scope are rejected and counted. FX framing setup for block operations is outside that scope. Tests do not time the overlays.',
        'universe_sha256': sha(UNIVERSE.read_bytes()),
        'harness_sha256': {p.name: sha(p.read_bytes()) for p in [Path(__file__), HERE / 'frozen_test.go.txt']},
        'baseline': run_target(args.baseline.resolve(), 'baseline', ids, build_flags),
        'candidate': run_target(args.candidate.resolve(), 'candidate', ids, build_flags),
    }
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + '\n')
    print('Wrote', output)


if __name__ == '__main__':
    main()
