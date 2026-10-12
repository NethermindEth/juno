#!/usr/bin/env python3
import html
import json
import statistics
import sys
from collections import namedtuple
from pathlib import Path

USAGE = """usage: compare.py <juno-dir> <trailseeker-dir> >report.html
Compare Juno and Trailseeker run-profiles.sh results as one HTML page.
  <dir>  one node's <workspaces>/<corpus>, holding <profile>/<trial>/report.md
Profiles follow plan.json order."""
PLAN = Path(__file__).with_name("plan.json")
TIE = 1.05
STEPS = (1.25, 2)
FAILED_ERRORS = 0.5
HIGHER = '<span class="arrow">▲</span>'
LOWER = '<span class="arrow">▼</span>'
Metric = namedtuple("Metric", "key tab description higher_better")
METRICS = (
    Metric("goodput", "Throughput", "Successful req/s; higher is better.", True),
    Metric("med", "p50 latency", "Median latency (ms); lower is better.", False),
    Metric("p95", "p95 latency", "p95 latency (ms); lower is better.", False),
)
COLUMNS = ("reqs", "rps", "err", "avg", "med", "p90", "p95", "p99", "max")
# profiles: {profile: {case: {"sets": [run], "warmup": [run]}}}; trials: {profile: count}
Node = namedtuple("Node", "name dir profiles trials events")


def read_report(path):
    for line in path.read_text().splitlines()[2:]:
        name, *values = (cell.strip() for cell in line.strip().strip("|").split("|"))
        case, run = name.rsplit(".", 1)
        row: dict[str, float] = dict(zip(COLUMNS, (float(v.rstrip("%")) for v in values)))
        row["err"] /= 100
        row["goodput"] = row["rps"] * (1 - row["err"])
        yield case, run, row


def load(name, root):
    """Pool each case's runs per profile across trials. A run without a single
    timed response means the node stopped answering: the cases that trial
    starts afterwards are dropped, since they no longer measure the node."""
    node = Node(name, root, {}, {}, [])
    for report in sorted(root.glob("*/*/report.md")):
        profile, trial = report.parent.parent.name, report.parent.name
        cases = node.profiles.setdefault(profile, {})
        node.trials[profile] = node.trials.get(profile, 0) + 1
        current, skip, down = None, False, False
        for case, run, row in read_report(report):
            entry = cases.setdefault(case, {"sets": [], "warmup": []})
            if case != current:
                current, skip = case, down
            if skip:
                continue
            if row["max"] == 0 and not down:
                down = True
                node.events.append(f"{name}, {profile} trial {trial}: stopped answering at {case}.{run}")
            entry["warmup" if run == "warmup" else "sets"].append(row)
    return node


def median(rows, key):
    return statistics.median(r[key] for r in rows)


def status(entry):
    if not entry["sets"]:
        return "na"
    if median(entry["sets"], "err") >= FAILED_ERRORS:
        return "failed"
    return "ok"


def verdict(juno, trailseeker, metric):
    """(winner, step, text); winner is j, t, tie or na, step the color depth."""
    js, ts = status(juno), status(trailseeker)
    if "na" in (js, ts):
        return "na", 0, "—"
    if js == ts == "failed":
        return "tie", 0, "both ✕"
    if js == "failed":
        return "t", 3, "✕"
    if ts == "failed":
        return "j", 3, "✕"
    j, t = median(juno["sets"], metric.key), median(trailseeker["sets"], metric.key)
    ratio = j / t if metric.higher_better else t / j
    gap = max(ratio, 1 / ratio)
    if gap < TIE:
        return "tie", 0, "≈"
    winner = "j" if ratio > 1 else "t"
    text = f"{HIGHER if j > t else LOWER} {gap:.3g}×"
    if any(r["err"] for r in juno["sets"] + trailseeker["sets"]):
        text += " !"
    return winner, 1 + sum(gap >= s for s in STEPS), text


def fmt(value):
    return f"{value:,.0f}" if value >= 100 else f"{value:.3g}"


def describe(name, entry, key):
    if not entry["sets"]:
        return f"{name}: not answering"
    values = [r[key] for r in entry["sets"]]
    text = f"{name}: {fmt(statistics.median(values))} ({fmt(min(values))}–{fmt(max(values))})"
    if entry["warmup"]:
        text += f", warmup {fmt(median(entry['warmup'], key))}"
    worst = max(r["err"] for r in entry["sets"])
    if worst:
        text += f", errors up to {worst:.2%}"
    return text


def section(index, metric, cases, profiles, juno, trailseeker, plan):
    score = {p: {"j": 0, "t": 0, "tie": 0, "na": 0} for p in profiles}
    rows, method = [], None
    for case in cases:
        if method and case.split("-")[0] != method:
            rows.append(f'<tr class="gap"><td colspan="{len(profiles) + 1}"></td></tr>')
        method = case.split("-")[0]
        cells = []
        for profile in profiles:
            j, t = juno.profiles[profile][case], trailseeker.profiles[profile][case]
            winner, step, text = verdict(j, t, metric)
            score[profile][winner] += 1
            tip = "&#10;".join(html.escape(describe(n.name, e, metric.key)) for n, e in ((juno, j), (trailseeker, t)))
            cells.append(f'<td class="{winner}{step}" title="{tip}">{text}</td>')
        rows.append(f"<tr><th>{html.escape(case)}</th>{''.join(cells)}</tr>")

    heads = "".join(
        f"<th>{p}<small>{html.escape(plan[p])}<br>"
        f"trials: Juno {juno.trials[p]}, Trailseeker {trailseeker.trials[p]}</small></th>"
        for p in profiles
    )
    totals = []
    for s in score.values():
        chips = (
            f'<span class="j2">Juno {s["j"]}</span> <span class="t2">Trailseeker {s["t"]}</span> '
            f'<span class="tie0">≈ {s["tie"]}</span>'
        )
        if s["na"]:
            chips += f' <span class="na0">— {s["na"]}</span>'
        totals.append(f'<td class="score">{chips}</td>')
    return f"""<section id="s{index}">
<p>{metric.description}</p>
<div class="scroll"><table>
<thead><tr><th>case</th>{heads}</tr><tr><th>wins</th>{''.join(totals)}</tr></thead>
<tbody>{''.join(rows)}</tbody>
</table></div>
</section>"""


def main():
    if len(sys.argv) != 3:
        sys.exit(USAGE)
    juno = load("Juno", Path(sys.argv[1]))
    trailseeker = load("Trailseeker", Path(sys.argv[2]))
    plan = json.loads(PLAN.read_text())["profiles"]
    profiles = [p for p in plan if p in juno.profiles and p in trailseeker.profiles]
    cases = list(juno.profiles[profiles[0]])

    tabs = "".join(
        f'<input type="radio" name="metric" id="m{i}"{" checked" if i == 0 else ""}><label for="m{i}">{m.tab}</label>'
        for i, m in enumerate(METRICS)
    )
    shown = ", ".join(f"#m{i}:checked ~ #s{i}" for i in range(len(METRICS)))
    sections = "".join(section(i, m, cases, profiles, juno, trailseeker, plan) for i, m in enumerate(METRICS))
    events = "".join(f"<li>{html.escape(e)}</li>" for e in trailseeker.events + juno.events)

    print(f"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Juno vs Trailseeker</title>
<style>
:root {{
  --bg: #fcfcfb; --ink: #1a1a19; --muted: #6b6a66; --line: #e2e1dd;
  --j1: #e3f4e3; --j2: #b4e0b4; --j3: #7cc67c;
  --t1: #fbe5e1; --t2: #f3bdb4; --t3: #e88a7c;
  --tie: #f0efec; --na: #d6d5d0;
}}
@media (prefers-color-scheme: dark) {{
  :root:not([data-theme="light"]) {{
    --bg: #1a1a19; --ink: #ecebe8; --muted: #a3a29d; --line: #383835;
    --j1: #1f3a22; --j2: #285a2e; --j3: #2e7536;
    --t1: #45201c; --t2: #6b2e26; --t3: #9c4237;
    --tie: #2b2b29; --na: #4a4a46;
  }}
}}
:root[data-theme="dark"] {{
  --bg: #1a1a19; --ink: #ecebe8; --muted: #a3a29d; --line: #383835;
  --j1: #1f3a22; --j2: #285a2e; --j3: #2e7536;
  --t1: #45201c; --t2: #6b2e26; --t3: #9c4237;
  --tie: #2b2b29; --na: #4a4a46;
}}
body {{ margin: 0 auto; padding: 16px; max-width: 1100px; background: var(--bg); color: var(--ink);
  font: 14px/1.4 system-ui, sans-serif; }}
h1 {{ font-size: 20px; margin: 0 0 4px; }}
h2 {{ font-size: 16px; margin: 24px 0 4px; }}
.sub, small, section > p {{ color: var(--muted); }}
input {{ position: absolute; opacity: 0; }}
label {{ display: inline-block; padding: 6px 12px; border: 1px solid var(--line); border-radius: 6px;
  margin: 12px 4px 0 0; cursor: pointer; }}
input:checked + label {{ background: var(--ink); color: var(--bg); }}
input:focus-visible + label {{ outline: 2px solid var(--ink); outline-offset: 2px; }}
section {{ display: none; }}
{shown} {{ display: block; }}
.scroll {{ overflow-x: auto; }}
table {{ border-collapse: separate; border-spacing: 2px; font-variant-numeric: tabular-nums; }}
th, td {{ padding: 4px 8px; white-space: nowrap; }}
thead th, thead td {{ white-space: normal; }}
th {{ text-align: left; font-weight: 500; }}
thead th {{ vertical-align: bottom; }}
tbody th, thead th:first-child {{ position: sticky; left: 0; background: var(--bg); }}
small {{ display: block; font-weight: 400; font-size: 11px; }}
td {{ text-align: center; min-width: 84px; border-radius: 4px; }}
tr.gap td {{ height: 6px; padding: 0; }}
.arrow {{ font-size: 0.5em; vertical-align: 0.35em; }}
td.score {{ font-size: 12px; }}
td.score span {{ display: inline-block; margin: 1px 0; padding: 1px 4px; border-radius: 4px; white-space: nowrap; }}
.j1 {{ background: var(--j1); }} .j2 {{ background: var(--j2); }} .j3 {{ background: var(--j3); }}
.t1 {{ background: var(--t1); }} .t2 {{ background: var(--t2); }} .t3 {{ background: var(--t3); }}
.tie0 {{ background: var(--tie); }}
.na0 {{ background: repeating-linear-gradient(45deg, var(--na) 0 3px, transparent 3px 7px); color: var(--muted); }}
ul {{ padding-left: 20px; }}
</style>
</head>
<body>
<h1>Juno vs Trailseeker</h1>
<p class="sub">Juno: {html.escape(str(juno.dir))}<br>Trailseeker: {html.escape(str(trailseeker.dir))}</p>
{tabs}
{sections}
<h2>How to read</h2>
<ul>
<li>Each cell compares the median of sets 1–5 over all trials. Warmup runs (seed 0, different data) are left out; hover a cell for them, the min–max range and errors.</li>
<li>{HIGHER} / {LOWER} Juno's value is higher / lower than Trailseeker's, by the shown factor. Color shows who wins: <span class="j2">Juno</span>, <span class="t2">Trailseeker</span>. <span class="tie0">≈</span> within ±{TIE - 1:.0%}. Darker means a gap of at least {STEPS[0]}× or {STEPS[1]}×.</li>
<li>Throughput counts successful requests only. <b>!</b> marks cells where some runs had errors.</li>
<li><b>✕</b> the losing node failed the case: at least {FAILED_ERRORS:.0%} errors in the median set.</li>
<li><span class="na0">—</span> not measured: the node had stopped answering before the case started.</li>
</ul>
{f"<h2>Unresponsive</h2><ul>{events}</ul>" if events else ""}
</body>
</html>""")


if __name__ == "__main__":
    main()
