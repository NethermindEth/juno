#!/usr/bin/env python3
"""Measure how quickly Juno nodes reflect the sequencer's pre-confirmed block.

Every ``--interval`` seconds the script asks the sequencer's feeder gateway for
its latest pre-confirmed block, using the same delta query Juno's poller sends,
and then immediately sends the same ``pre_confirmed`` JSON-RPC request to every
node in parallel, recording each node's request latency and the block number
and transaction count it returned.

The report covers, per node: request latency percentiles; how often the node
already had the sequencer's state when asked; how long a client had to wait
until the node returned each new sequencer state ("time to see", which includes
the request latency); which node returned each new state first; and, when
``--metrics`` is given, the gateway calls the node made during the measurement
window.

Usage:

    python3 bench/preconfirmed/preconfirmed_bench.py \\
        --node baseline=http://localhost:6060 \\
        --node ondemand=http://localhost:6061 \\
        --metrics baseline=http://localhost:9090/metrics \\
        --metrics ondemand=http://localhost:9091/metrics \\
        --api-key "$GW_API_KEY" --interval 0.2 --duration 300

Standard library only; Python 3.9+.
"""
from __future__ import annotations

import argparse
import csv
import http.client
import json
import math
import re
import socket
import statistics
import sys
import time
import urllib.parse
import urllib.request
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional

DEFAULT_SEQUENCER = "https://feeder.alpha-mainnet.starknet.io/feeder_gateway"
DEFAULT_RPC_PATH = "/v0_10"
BLANK_IDENTIFIER = "0x0"
GATEWAY_ENDPOINT = "get_preconfirmed_block"
GATEWAY_CALLS_METRIC = "feeder_client_request_latency_count"
METRIC_LINE = re.compile(r"^(\w+)\{([^}]*)\}\s+(\S+)")
METRIC_LABEL = re.compile(r'(\w+)="([^"]*)"')
SAME_TIME_MS = 1.0  # answers closer than this count as a tie for "saw first"
PROGRESS_EVERY_S = 10.0
MAX_SYNC_LAG_BLOCKS = 2


class RequestError(Exception):
    """An HTTP or JSON-RPC level failure; the message is what lands in the CSV."""


@dataclass(frozen=True)
class ChainState:
    """A pre-confirmed block seen as (block number, transaction count)."""

    block: int
    txs: int

    def at_least(self, other: ChainState) -> bool:
        return self.block > other.block or (self.block == other.block and self.txs >= other.txs)

    def newer_than(self, other: ChainState) -> bool:
        return self.block > other.block or (self.block == other.block and self.txs > other.txs)

    def label(self) -> str:
        return f"{self.block} ({self.txs} txs)"


@dataclass
class SequencerSample:
    t_done: float  # seconds since run start when the reply was fully read
    rtt_ms: float
    status: int  # HTTP status, 0 on transport error
    state: Optional[ChainState]  # None when unknown this iteration
    error: str = ""


@dataclass
class Probe:
    t_sent: float
    t_recv: float
    latency_ms: float
    state: Optional[ChainState]
    error: str = ""


@dataclass
class Row:
    index: int
    t: float
    seq: SequencerSample
    probes: dict[str, Probe]


class KeepAliveClient:
    """HTTP/1.1 client reusing one connection, so latency excludes TCP/TLS setup.

    Not thread-safe: use one instance per target and never concurrently.
    """

    _RETRIABLE = (http.client.RemoteDisconnected, BrokenPipeError, ConnectionResetError)

    def __init__(self, base_url: str, timeout: float, headers: dict[str, str]) -> None:
        parts = urllib.parse.urlsplit(base_url)
        if parts.scheme not in ("http", "https") or not parts.hostname:
            raise ValueError(f"unsupported URL {base_url!r}")
        self._scheme = parts.scheme
        self._host = parts.hostname
        self._port = parts.port
        self._timeout = timeout
        self._headers = headers
        self._conn: Optional[http.client.HTTPConnection] = None
        self.reconnects = 0

    def close(self) -> None:
        if self._conn is not None:
            self._conn.close()
            self._conn = None

    def request(self, method: str, path: str, body: Optional[bytes] = None) -> tuple[int, bytes]:
        """Perform one request and return (status, body).

        A connection the server dropped while idle is reopened once; anything
        else propagates to the caller as an OSError or http.client.HTTPException.
        """
        for attempt in (0, 1):
            if self._conn is None:
                conn_cls = (
                    http.client.HTTPSConnection
                    if self._scheme == "https"
                    else http.client.HTTPConnection
                )
                self._conn = conn_cls(self._host, self._port, timeout=self._timeout)
                self._conn.connect()
                # Send each request as one segment: no Nagle stall behind a delayed ACK.
                self._conn.sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
            try:
                self._conn.request(method, path, body=body, headers=self._headers)
                response = self._conn.getresponse()
                data = response.read()
            except self._RETRIABLE:
                self.close()
                if attempt == 1:
                    raise
                self.reconnects += 1
                continue
            except (http.client.HTTPException, OSError):
                self.close()
                raise
            if response.getheader("Connection", "").lower() == "close":
                self.close()
            return response.status, data
        raise AssertionError("unreachable")


class SequencerClient:
    """Polls get_preconfirmed_block with delta hints, exactly like Juno's poller."""

    def __init__(self, base_url: str, api_key: str, timeout: float) -> None:
        self.base_url = base_url.rstrip("/")
        self._path_prefix = urllib.parse.urlsplit(self.base_url).path
        headers = {"User-Agent": "juno-preconfirmed-bench", "Accept": "application/json"}
        if api_key:
            headers["X-Throttling-Bypass"] = api_key
        self._http = KeepAliveClient(self.base_url, timeout, headers)
        self._identifier = BLANK_IDENTIFIER
        self._state: Optional[ChainState] = None

    def poll(self, t0: float) -> SequencerSample:
        query = urllib.parse.urlencode(
            {
                "blockNumber": "latest",
                "blockIdentifier": self._identifier,
                "knownTransactionCount": self._state.txs if self._state else 0,
            }
        )
        sent = time.perf_counter()
        try:
            status, body = self._http.request(
                "GET", f"{self._path_prefix}/{GATEWAY_ENDPOINT}?{query}"
            )
        except (OSError, http.client.HTTPException) as exc:
            done = time.perf_counter()
            return SequencerSample(done - t0, (done - sent) * 1000, 0, None, describe(exc))
        done = time.perf_counter()
        rtt_ms = (done - sent) * 1000
        if status == 400:  # the gateway has no pre-confirmed block right now
            self._identifier, self._state = BLANK_IDENTIFIER, None
            return SequencerSample(done - t0, rtt_ms, status, None)
        if status != 200:
            return SequencerSample(done - t0, rtt_ms, status, None, f"HTTP {status}")
        try:
            self._apply(json.loads(body))
        except (ValueError, KeyError, TypeError) as exc:
            self._identifier, self._state = BLANK_IDENTIFIER, None
            return SequencerSample(done - t0, rtt_ms, status, None, f"bad payload: {describe(exc)}")
        return SequencerSample(done - t0, rtt_ms, status, self._state)

    def _apply(self, payload: dict[str, Any]) -> None:
        if not payload.get("changed", True):
            return  # {"changed": false}: what we know is still current
        txs = payload.get("transactions") or []
        if payload.get("timestamp"):  # a full block: new round, or our first look
            self._identifier = payload["block_identifier"]
            self._state = ChainState(int(payload["block_number"]), len(txs))
            return
        if self._state is None:
            raise ValueError("delta received without a known round")
        # A delta only carries the transactions appended since knownTransactionCount.
        block = int(payload.get("block_number", self._state.block))
        self._state = ChainState(block, self._state.txs + len(txs))


class NodeClient:
    """JSON-RPC client for one Juno node."""

    def __init__(self, name: str, url: str, method: str, timeout: float) -> None:
        parts = urllib.parse.urlsplit(url)
        self.name = name
        self.url = url
        self.method = method
        self._path = parts.path if parts.path not in ("", "/") else DEFAULT_RPC_PATH
        if parts.query:
            self._path += "?" + parts.query
        self._http = KeepAliveClient(
            f"{parts.scheme}://{parts.netloc}",
            timeout,
            {"Content-Type": "application/json", "Accept": "application/json"},
        )

    def call(self, method: str, params: Any) -> Any:
        body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
        try:
            status, data = self._http.request("POST", self._path, body.encode())
        except (OSError, http.client.HTTPException) as exc:
            raise RequestError(describe(exc)) from exc
        if status != 200:
            raise RequestError(f"HTTP {status}: {data[:120]!r}")
        try:
            payload = json.loads(data)
        except ValueError as exc:
            raise RequestError(f"bad JSON: {exc}") from exc
        if "error" in payload:
            error = payload["error"]
            raise RequestError(f"rpc error {error.get('code')}: {error.get('message')}")
        return payload.get("result")

    def probe(self, t0: float) -> Probe:
        """Fetch the pre-confirmed block and time the round-trip."""
        sent = time.perf_counter()
        try:
            result = self.call(self.method, {"block_id": "pre_confirmed"})
            state: Optional[ChainState] = ChainState(
                int(result["block_number"]), len(result["transactions"])
            )
            error = ""
        except (RequestError, KeyError, TypeError, ValueError) as exc:
            state, error = None, describe(exc)
        recv = time.perf_counter()
        return Probe(sent - t0, recv - t0, (recv - sent) * 1000, state, error)


def describe(exc: BaseException) -> str:
    text = str(exc).strip()
    return f"{type(exc).__name__}: {text}" if text else type(exc).__name__


def parse_named(values: list[str], flag: str) -> dict[str, str]:
    """Parse repeated NAME=URL flags into an ordered dict."""
    out: dict[str, str] = {}
    for value in values:
        name, sep, url = value.partition("=")
        if not sep or not name or not url:
            sys.exit(f"{flag} expects NAME=URL, got {value!r}")
        if name in out:
            sys.exit(f"duplicate {flag} name {name!r}")
        out[name] = url
    return out


def scrape_gateway_calls(url: str, timeout: float) -> dict[str, int]:
    """Return {http status: count} of get_preconfirmed_block calls from a /metrics page."""
    with urllib.request.urlopen(url, timeout=timeout) as response:
        text = response.read().decode()
    counts: dict[str, int] = {}
    for line in text.splitlines():
        match = METRIC_LINE.match(line)
        if not match or match.group(1) != GATEWAY_CALLS_METRIC:
            continue
        labels = dict(METRIC_LABEL.findall(match.group(2)))
        if GATEWAY_ENDPOINT not in labels.get("method", ""):
            continue
        status = labels.get("status", "?")
        counts[status] = counts.get(status, 0) + int(float(match.group(3)))
    return counts


def scrape_all(metrics: dict[str, str], timeout: float) -> dict[str, dict[str, int]]:
    result = {}
    for name, url in metrics.items():
        try:
            result[name] = scrape_gateway_calls(url, timeout)
        except (OSError, ValueError, http.client.HTTPException) as exc:
            print(f"warning: metrics for {name} unavailable ({describe(exc)})", file=sys.stderr)
    return result


def preflight(seq: SequencerClient, nodes: list[NodeClient]) -> dict[str, str]:
    """Check every endpoint answers and every node is synced; return node versions."""
    sample = seq.poll(time.perf_counter())
    if sample.error:
        sys.exit(f"sequencer {seq.base_url}: {sample.error}")
    if sample.state is None:
        print("sequencer: no pre-confirmed block in the gateway window right now", file=sys.stderr)
    else:
        print(
            f"sequencer: pre-confirmed block {sample.state.label()}"
            f" (rtt {sample.rtt_ms:.0f} ms)",
            file=sys.stderr,
        )

    chain_ids: dict[str, str] = {}
    versions: dict[str, str] = {}
    for node in nodes:
        try:
            chain_ids[node.name] = str(node.call("starknet_chainId", []))
            try:
                versions[node.name] = str(node.call("juno_version", []))
            except RequestError:
                versions[node.name] = "unknown"
            head = int(node.call("starknet_blockNumber", []))
            probe = node.probe(time.perf_counter())
        except RequestError as exc:
            sys.exit(f"{node.name} ({node.url}): {exc}")
        if probe.error:
            sys.exit(f"{node.name} ({node.url}): {node.method} pre_confirmed failed: {probe.error}")
        if sample.state is not None and head < sample.state.block - MAX_SYNC_LAG_BLOCKS:
            behind = sample.state.block - 1 - head
            sys.exit(f"{node.name} is {behind} blocks behind the sequencer; wait for it to sync")
        print(
            f"{node.name}: {versions[node.name]}, head {head},"
            f" pre-confirmed {probe.state.label() if probe.state else '?'}"
            f" ({probe.latency_ms:.0f} ms)",
            file=sys.stderr,
        )
    if len(set(chain_ids.values())) > 1:
        sys.exit(f"nodes are on different chains: {chain_ids}")
    if len(nodes) > 1 and len(set(versions.values())) == 1:
        print("warning: every node reports the same juno_version", file=sys.stderr)
    return versions


def csv_header(names: list[str]) -> list[str]:
    columns = ["iteration", "t_s", "seq_t_done_s", "seq_block", "seq_txs", "seq_status",
               "seq_rtt_ms", "seq_error"]
    for name in names:
        columns += [f"{name}_t_sent_s", f"{name}_t_recv_s", f"{name}_latency_ms",
                    f"{name}_block", f"{name}_txs", f"{name}_error"]
    return columns


def csv_row(row: Row, names: list[str]) -> list[Any]:
    seq = row.seq
    values: list[Any] = [
        row.index, f"{row.t:.4f}", f"{seq.t_done:.4f}",
        seq.state.block if seq.state else "", seq.state.txs if seq.state else "",
        seq.status, f"{seq.rtt_ms:.1f}", seq.error,
    ]
    for name in names:
        probe = row.probes[name]
        values += [
            f"{probe.t_sent:.4f}", f"{probe.t_recv:.4f}", f"{probe.latency_ms:.1f}",
            probe.state.block if probe.state else "", probe.state.txs if probe.state else "",
            probe.error,
        ]
    return values


@dataclass
class NodeStats:
    requests: int = 0
    errors: int = 0
    latencies: list[float] = field(default_factory=list)
    compared: int = 0
    caught_up: int = 0
    ahead: int = 0
    txs_behind: list[int] = field(default_factory=list)
    block_lag: int = 0
    time_to_see_ms: list[float] = field(default_factory=list)
    unseen: int = 0
    saw_first: int = 0
    gateway_calls: Optional[dict[str, int]] = None


@dataclass
class SequencerStats:
    samples: int = 0
    errors: int = 0
    no_window: int = 0
    rtts: list[float] = field(default_factory=list)
    events: int = 0
    block_rolls: int = 0
    txs_seen: int = 0
    seconds: float = 0.0


def evaluate(
    rows: list[Row], names: list[str], warmup: float
) -> tuple[dict[str, NodeStats], SequencerStats]:
    """Compute per-node and sequencer statistics over the rows past warm-up."""
    measured = [row for row in rows if row.t >= warmup]
    stats = {name: NodeStats() for name in names}
    seq_stats = SequencerStats(samples=len(measured))
    if not measured:
        return stats, seq_stats
    seq_stats.seconds = measured[-1].seq.t_done - measured[0].t

    for row in measured:
        seq = row.seq
        if seq.error:
            seq_stats.errors += 1
        elif seq.state is None:
            seq_stats.no_window += 1
        if seq.status == 200:
            seq_stats.rtts.append(seq.rtt_ms)
        for name in names:
            probe, node = row.probes[name], stats[name]
            node.requests += 1
            if probe.state is None:
                node.errors += 1
                continue
            node.latencies.append(probe.latency_ms)
            if seq.state is None:
                continue
            node.compared += 1
            if probe.state.at_least(seq.state):
                node.caught_up += 1
                if probe.state.newer_than(seq.state):
                    node.ahead += 1
            if probe.state.block == seq.state.block:
                node.txs_behind.append(max(0, seq.state.txs - probe.state.txs))
            elif probe.state.block < seq.state.block:
                node.block_lag += 1

    # New sequencer states: every block roll or transaction-count increase. The
    # last state observed during warm-up seeds the comparison so the first
    # measured sample does not count as an event by itself.
    previous: Optional[ChainState] = None
    for row in rows:
        if row.t >= warmup:
            break
        if row.seq.state is not None:
            previous = row.seq.state
    for i, row in enumerate(measured):
        state = row.seq.state
        if state is None or (previous is not None and not state.newer_than(previous)):
            continue
        if previous is not None:
            seq_stats.events += 1
            if state.block > previous.block:
                seq_stats.block_rolls += 1
                seq_stats.txs_seen += state.txs
            else:
                seq_stats.txs_seen += state.txs - previous.txs
            first_seen = {}
            for name in names:
                for later in measured[i:]:
                    probe = later.probes[name]
                    if probe.state is not None and probe.state.at_least(state):
                        first_seen[name] = probe.t_recv
                        stats[name].time_to_see_ms.append((probe.t_recv - row.seq.t_done) * 1000)
                        break
                else:
                    stats[name].unseen += 1
            if first_seen:
                earliest = min(first_seen.values())
                for name, t_recv in first_seen.items():
                    if (t_recv - earliest) * 1000 <= SAME_TIME_MS:
                        stats[name].saw_first += 1
        previous = state
    return stats, seq_stats


def percentile(values: list[float], pct: float) -> float:
    ordered = sorted(values)
    rank = max(1, math.ceil(pct / 100 * len(ordered)))
    return ordered[rank - 1]


def fmt_ms(value: float) -> str:
    return f"{value:.1f}" if value < 10 else f"{value:.0f}"


def fmt_pcts(values: list[float], pcts: tuple[float, ...] = (50, 90, 100)) -> str:
    if not values:
        return "n/a"
    return "/".join(fmt_ms(percentile(values, p)) for p in pcts)


def fmt_share(part: int, whole: int) -> str:
    return f"{100 * part / whole:.1f}%" if whole else "n/a"


def build_report(
    args: argparse.Namespace,
    started_at: datetime,
    names: list[str],
    versions: dict[str, str],
    seq: SequencerClient,
    stats: dict[str, NodeStats],
    seq_stats: SequencerStats,
    gw_start: dict[str, dict[str, int]],
    gw_end: dict[str, dict[str, int]],
) -> str:
    lines = ["# Pre-confirmed freshness benchmark", ""]
    rate = seq_stats.samples / seq_stats.seconds if seq_stats.seconds else 0.0
    lines.append(
        f"- started {started_at.strftime('%Y-%m-%dT%H:%M:%SZ')}; measured"
        f" {seq_stats.seconds:.0f} s after a {args.warmup:g} s warm-up;"
        f" {seq_stats.samples} iterations at {rate:.1f}/s (asked for {1 / args.interval:.1f}/s;"
        " an iteration waits for the slowest node); probe `" + args.method + "`"
    )
    tx_rate = seq_stats.txs_seen / seq_stats.seconds if seq_stats.seconds else 0.0
    lines.append(
        f"- sequencer `{seq.base_url}`: {seq_stats.samples} samples, {seq_stats.errors} errors,"
        f" {seq_stats.no_window} without a pre-confirmed block; {seq_stats.events} new states"
        f" ({seq_stats.block_rolls} block rolls, {tx_rate:.1f} tx/s observed);"
        f" rtt p50/p90 {fmt_pcts(seq_stats.rtts, (50, 90))} ms"
    )
    lines.append(
        "- caught up: the node's answer was at least the sequencer state read just before"
        " asking. time to see: from the sequencer's reply showing a new state until the"
        f" client received that state (or newer) from the node; resolution {args.interval:g} s"
        " plus request latency. saw first: share of new states this node returned before the"
        " others."
    )
    lines.append("")
    header = (
        "| node | version | reqs | errors | lat avg (ms) | p50 | p90 | p99 | max | caught up"
        " | ahead | txs behind p50/p90/max | block lag | time to see p50/p90/max (ms) | unseen"
        " | saw first | gw calls | gw/req | gw non-200 |"
    )
    lines.append(header)
    lines.append("|" + " --- |" * (header.count("|") - 1))
    for name in names:
        node = stats[name]
        lat = node.latencies
        if name in gw_start and name in gw_end:
            calls = {
                status: gw_end[name].get(status, 0) - gw_start[name].get(status, 0)
                for status in set(gw_start[name]) | set(gw_end[name])
            }
            total = sum(calls.values())
            bad = sum(count for status, count in calls.items() if status != "200")
            per_request = f"{total / node.requests:.2f}" if node.requests else "n/a"
            gw_cells = f"{total} | {per_request} | {bad}"
        else:
            gw_cells = "n/a | n/a | n/a"
        lines.append(
            f"| {name} | {versions.get(name, '?')} | {node.requests} | {node.errors}"
            f" | {fmt_ms(statistics.fmean(lat)) if lat else 'n/a'}"
            f" | {fmt_pcts(lat, (50,))} | {fmt_pcts(lat, (90,))} | {fmt_pcts(lat, (99,))}"
            f" | {fmt_ms(max(lat)) if lat else 'n/a'}"
            f" | {fmt_share(node.caught_up, node.compared)}"
            f" | {fmt_share(node.ahead, node.compared)}"
            f" | {fmt_pcts([float(v) for v in node.txs_behind])} | {node.block_lag}"
            f" | {fmt_pcts(node.time_to_see_ms)} | {node.unseen}"
            f" | {fmt_share(node.saw_first, seq_stats.events)} | {gw_cells} |"
        )
    lines.append("")
    return "\n".join(lines)


def run(args: argparse.Namespace) -> int:
    node_urls = parse_named(args.node, "--node")
    metrics = parse_named(args.metrics, "--metrics")
    unknown = set(metrics) - set(node_urls)
    if unknown:
        sys.exit(f"--metrics names without a --node: {sorted(unknown)}")
    if not node_urls:
        sys.exit("at least one --node NAME=URL is required")
    nodes = [NodeClient(name, url, args.method, args.timeout) for name, url in node_urls.items()]
    names = [node.name for node in nodes]
    seq = SequencerClient(args.sequencer, args.api_key, args.timeout)
    versions = preflight(seq, nodes)

    started_at = datetime.now(timezone.utc)
    stamp = started_at.strftime("%Y%m%dT%H%M%SZ")
    out_dir = Path(args.out or f"bench/preconfirmed/results/{stamp}")
    out_dir.mkdir(parents=True, exist_ok=True)
    rows: list[Row] = []
    gw_start: dict[str, dict[str, int]] = {}
    gw_end: dict[str, dict[str, int]] = {}
    measuring = False
    t0 = time.perf_counter()
    next_tick = t0
    next_progress = t0 + PROGRESS_EVERY_S
    print(f"running for {args.duration:g} s; results in {out_dir}", file=sys.stderr)
    with (out_dir / "samples.csv").open("w", newline="") as fh, ThreadPoolExecutor(
        max_workers=len(nodes)
    ) as pool:
        writer = csv.writer(fh)
        writer.writerow(csv_header(names))
        try:
            while True:
                now = time.perf_counter()
                if now - t0 >= args.duration:
                    break
                if now < next_tick:
                    time.sleep(next_tick - now)
                # Never schedule in the past: an overrun shifts later ticks instead of bursting.
                next_tick = max(next_tick + args.interval, time.perf_counter())
                t = time.perf_counter() - t0
                if not measuring and t >= args.warmup:
                    measuring = True
                    gw_start = scrape_all(metrics, args.timeout)
                sample = seq.poll(t0)
                futures = [pool.submit(node.probe, t0) for node in nodes]
                probes = {node.name: future.result() for node, future in zip(nodes, futures)}
                row = Row(len(rows), t, sample, probes)
                rows.append(row)
                writer.writerow(csv_row(row, names))
                fh.flush()
                if time.perf_counter() >= next_progress:
                    next_progress += PROGRESS_EVERY_S
                    seen = ", ".join(
                        f"{name}={p.state.label() if p.state else p.error}"
                        for name, p in probes.items()
                    )
                    seq_seen = (
                        sample.state.label() if sample.state else (sample.error or "no window")
                    )
                    print(f"t={t:6.1f}s sequencer={seq_seen} {seen}", file=sys.stderr)
        except KeyboardInterrupt:
            print("interrupted; writing the report", file=sys.stderr)
        if measuring:
            gw_end = scrape_all(metrics, args.timeout)

    stats, seq_stats = evaluate(rows, names, args.warmup)
    report = build_report(
        args, started_at, names, versions, seq, stats, seq_stats, gw_start, gw_end
    )
    (out_dir / "report.md").write_text(report)
    print(report)
    print(f"samples: {out_dir / 'samples.csv'}\nreport:  {out_dir / 'report.md'}", file=sys.stderr)
    return 0


def parse_args(argv: Optional[list[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=__doc__.split("\n\n")[0],
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument(
        "--node", action="append", default=[], metavar="NAME=URL",
        help="Juno JSON-RPC endpoint; repeatable. A bare host:port gets /v0_10 appended.",
    )
    parser.add_argument(
        "--metrics", action="append", default=[], metavar="NAME=URL",
        help="Juno Prometheus /metrics URL for a node named by --node; repeatable, optional.",
    )
    parser.add_argument("--sequencer", default=DEFAULT_SEQUENCER, help="feeder gateway base URL")
    parser.add_argument("--api-key", default="", help="X-Throttling-Bypass for sequencer calls")
    parser.add_argument(
        "--method", default="starknet_getBlockWithTxHashes",
        choices=["starknet_getBlockWithTxHashes", "starknet_getBlockWithTxs"],
        help="JSON-RPC method used to read the pre-confirmed block",
    )
    parser.add_argument("--interval", type=float, default=0.2, help="seconds between iterations")
    parser.add_argument("--duration", type=float, default=300.0, help="total run time in seconds")
    parser.add_argument("--warmup", type=float, default=15.0, help="seconds excluded from stats")
    parser.add_argument("--timeout", type=float, default=5.0, help="per-request timeout, seconds")
    parser.add_argument("--out", default="", help="output directory (default: results/<timestamp>)")
    args = parser.parse_args(argv)
    if args.interval <= 0 or args.duration <= 0 or args.timeout <= 0 or args.warmup < 0:
        parser.error("--interval, --duration and --timeout must be positive; --warmup >= 0")
    if args.warmup >= args.duration:
        parser.error("--warmup must be shorter than --duration")
    return args


if __name__ == "__main__":
    sys.exit(run(parse_args()))
