#!/usr/bin/env python3
"""Measure how quickly Juno nodes reflect the sequencer's pre-confirmed block.

Every ``--interval`` seconds the script asks the sequencer's feeder gateway for
its latest pre-confirmed block, using the same delta query Juno's poller sends.
Independently, each node gets the same ``pre_confirmed`` JSON-RPC request at
random (Poisson) times with a mean gap of ``--interval``. Each request goes out
on schedule whether or not the node has answered the earlier ones (up to
``--max-in-flight`` per node), so every node is sampled at the same rate whatever
its latency. The script records the request latency and the block returned
(number and transaction hashes). No node's latency changes when the gateway or
another node is sampled.

The report covers, per node: the achieved request rate and latency percentiles;
how often the node already had the newest sequencer state the script knew of
when it asked; how long a client had to wait until the node returned each new
sequencer state ("time to see", which includes the request latency); which node
returned each new state first; and, when ``--metrics`` is given, the gateway
responses the node's feeder client got during the measurement window.

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
import bisect
import contextlib
import csv
import http.client
import itertools
import json
import math
import queue
import random
import re
import select
import signal
import socket
import statistics
import sys
import threading
import time
import urllib.parse
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Iterator, Optional

DEFAULT_SEQUENCER = "https://feeder.alpha-mainnet.starknet.io/feeder_gateway"
DEFAULT_RPC_PATH = "/v0_10"
DEFAULT_METRICS_PATH = "/metrics"
BLANK_IDENTIFIER = "0x0"
GATEWAY_ENDPOINT = "get_preconfirmed_block"
GATEWAY_RESPONSES_METRIC = "feeder_client_request_latency_count"
METRIC_LINE = re.compile(r"^(\w+)\{([^}]*)\}\s+(\S+)")
METRIC_LABEL = re.compile(r'(\w+)="([^"]*)"')
SAME_TIME_MS = 1.0  # answers closer than this count as a tie for "saw first"
PROGRESS_EVERY_S = 10.0
MAX_SYNC_LAG_BLOCKS = 2
SEQUENCER = "sequencer"  # the source name of the script's own gateway reads in samples.csv


class RequestError(Exception):
    """An HTTP or JSON-RPC level failure; the message is what lands in the CSV."""


@dataclass(frozen=True)
class ChainState:
    """A pre-confirmed block: its number, its transaction hashes in order, and its round.

    ``identifier`` is the gateway's ``block_identifier``, which changes when a new round
    replaces the block at the same height. Only the script's own gateway reads carry it: a
    node's JSON-RPC answer has none, so node states leave it empty and are placed in a round
    by their transaction hashes (see RoundBook).
    """

    block: int
    txs: tuple[int, ...]
    identifier: str = ""

    def label(self) -> str:
        return f"{self.block} ({len(self.txs)} txs)"


@dataclass
class SequencerSample:
    t_sent: float  # seconds since run start when the request was sent
    t_done: float  # seconds since run start when the reply was fully read
    rtt_ms: float
    status: int  # HTTP status, 0 on transport error
    state: Optional[ChainState]  # None when unknown: an error, or a 400 (no pre-confirmed block)
    error: str = ""


@dataclass
class Probe:
    t_sent: float
    t_recv: float
    latency_ms: float
    state: Optional[ChainState]
    error: str = ""
    in_flight: int = 1  # requests to the node outstanding when it was sent, itself included


_HASHES: dict[int, int] = {}


def tx_hashes(transactions: list[Any]) -> tuple[int, ...]:
    """Transaction hashes as ints, from hash strings or objects with a ``transaction_hash``.

    The gateway pads hashes with zeros and Juno does not, so they are compared as numbers.
    Each distinct hash is stored once, since a run keeps every state it sees.
    """
    hashes = []
    for tx in transactions:
        value = int(tx if isinstance(tx, str) else tx["transaction_hash"], 16)
        hashes.append(_HASHES.setdefault(value, value))
    return tuple(hashes)


def with_scheme(url: str) -> str:
    """Treat a bare host:port as http://host:port."""
    return url if "://" in url else f"http://{url}"


class KeepAliveClient:
    """HTTP/1.1 client reusing one connection, so latency excludes TCP/TLS setup.

    It connects directly and ignores HTTP(S)_PROXY / NO_PROXY, like every request the script
    makes. Not thread-safe: one request at a time per instance (NodeClient keeps a pool).
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
        self.connects = 0
        self.reconnects = 0
        self.sent_at = 0.0  # perf_counter when the last request went out on an open connection

    def close(self) -> None:
        if self._conn is not None:
            self._conn.close()
            self._conn = None

    def connect(self) -> None:
        """Open the connection, unless one is open that the server has not closed.

        A server closes an idle keep-alive connection (Juno's after 2 minutes) by shutting it
        down, which makes it readable while no request is outstanding: reopen it now rather
        than fail a request on it.
        """
        if self._conn is not None:
            sock = self._conn.sock
            try:
                if sock is not None and not select.select([sock], [], [], 0)[0]:
                    return
            except ValueError:  # a descriptor past select()'s limit: no way to tell, keep it
                return
            except OSError:
                pass
            self.close()
        conn_cls = (
            http.client.HTTPSConnection if self._scheme == "https" else http.client.HTTPConnection
        )
        conn = conn_cls(self._host, self._port, timeout=self._timeout)
        conn.connect()
        # Send each request as one segment: no Nagle stall behind a delayed ACK. http.client's
        # connect() already sets TCP_NODELAY (3.9+); this keeps it explicit.
        conn.sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
        self._conn = conn
        self.connects += 1

    def request(self, method: str, path: str, body: Optional[bytes] = None) -> tuple[int, bytes]:
        """Perform one request and return (status, body).

        A connection the server dropped while idle is reopened once; anything
        else propagates to the caller as an OSError or http.client.HTTPException.
        ``sent_at`` is when the request went out on an open connection (when the call began,
        if connecting failed), so a caller can leave connection setup out of its latency.
        """
        self.sent_at = time.perf_counter()
        for attempt in (0, 1):
            self.connect()
            self.sent_at = time.perf_counter()
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
    """Polls get_preconfirmed_block with delta hints, exactly like Juno's poller.

    The known state only changes when a reply applies, as Juno's chain only changes when an
    update applies: a 400 (no pre-confirmed block in the gateway window), another status, a
    transport error or a reply Juno's decoder rejects all keep it, so the next poll sends the
    same blockIdentifier and knownTransactionCount again.
    """

    def __init__(self, base_url: str, api_key: str, timeout: float) -> None:
        self.base_url = base_url.rstrip("/")
        self._path_prefix = urllib.parse.urlsplit(self.base_url).path
        headers = {"User-Agent": "juno-preconfirmed-bench", "Accept": "application/json"}
        if api_key:
            headers["X-Throttling-Bypass"] = api_key
        self._http = KeepAliveClient(self.base_url, timeout, headers)
        self._state: Optional[ChainState] = None

    def latest_block_number(self) -> int:
        """Number of the gateway's latest block, the head Juno's sync compares itself with."""
        status, body = self._http.request(
            "GET", f"{self._path_prefix}/get_block?blockNumber=latest&headerOnly=true"
        )
        if status != 200:
            raise RequestError(f"get_block latest: HTTP {status}")
        try:
            return int(json.loads(body)["block_number"])
        except (ValueError, KeyError, TypeError) as exc:
            raise RequestError(f"get_block latest: bad payload: {describe(exc)}") from exc

    def poll(self, t0: float) -> SequencerSample:
        query = urllib.parse.urlencode(
            {
                "blockNumber": "latest",
                "blockIdentifier": self._state.identifier if self._state else BLANK_IDENTIFIER,
                "knownTransactionCount": len(self._state.txs) if self._state else 0,
            }
        )
        sent = time.perf_counter()
        try:
            status, body = self._http.request(
                "GET", f"{self._path_prefix}/{GATEWAY_ENDPOINT}?{query}"
            )
        except (OSError, http.client.HTTPException) as exc:
            done = time.perf_counter()
            return SequencerSample(
                sent - t0, done - t0, (done - sent) * 1000, 0, None, describe(exc)
            )
        done = time.perf_counter()
        rtt_ms = (done - sent) * 1000
        if status == 400:  # no pre-confirmed block in the gateway window; Juno skips the tick
            return SequencerSample(sent - t0, done - t0, rtt_ms, status, None)
        if status != 200:
            return SequencerSample(sent - t0, done - t0, rtt_ms, status, None, f"HTTP {status}")
        try:
            self._state = self._apply(json.loads(body))
        except (ValueError, KeyError, TypeError) as exc:
            return SequencerSample(
                sent - t0, done - t0, rtt_ms, status, None, f"bad payload: {describe(exc)}"
            )
        return SequencerSample(sent - t0, done - t0, rtt_ms, status, self._state)

    def _apply(self, payload: dict[str, Any]) -> ChainState:
        """Return the state after one reply, raising where Juno's decoder or chain would."""
        if "changed" not in payload:
            raise ValueError('missing required "changed" field')
        state = self._state
        if not payload["changed"]:
            if state is None:
                raise ValueError("no-change reply without a known block")
            return state  # {"changed": false}: what we know is still current
        txs = tx_hashes(payload.get("transactions") or [])
        if payload.get("timestamp"):  # a full block: a new block or round, or our first look
            block = int(payload["block_number"])
            if not block:
                raise ValueError("full block without block_number")
            identifier = payload["block_identifier"]
            # shouldPreserveSlot: the same round without extra transactions keeps what we have.
            if (
                state is not None
                and (state.block, state.identifier) == (block, identifier)
                and len(txs) <= len(state.txs)
            ):
                return state
            return ChainState(block, txs, identifier)
        if state is None:
            raise ValueError("delta received without a known round")
        if not txs:
            raise ValueError("delta without transactions")
        # A delta only carries the transactions appended since knownTransactionCount; Juno
        # applies it to its most recent block whatever block_number the reply carries.
        return ChainState(state.block, state.txs + txs, state.identifier)


class NodeClient:
    """JSON-RPC client for one Juno node, safe to call from several threads at once.

    An http.client connection carries one request at a time, so each request in flight gets
    its own keep-alive connection from a pool. The connection returned last goes out first:
    at low concurrency the same one or two carry every request and stay warm, and the pool
    holds as many connections as there were requests in flight at once.
    """

    def __init__(self, name: str, url: str, method: str, timeout: float) -> None:
        parts = urllib.parse.urlsplit(with_scheme(url))
        self.name = name
        self.url = url
        self.method = method
        self._path = parts.path if parts.path not in ("", "/") else DEFAULT_RPC_PATH
        if parts.query:
            self._path += "?" + parts.query
        self._base = f"{parts.scheme}://{parts.netloc}"
        self._timeout = timeout
        self._headers = {"Content-Type": "application/json", "Accept": "application/json"}
        first = KeepAliveClient(self._base, timeout, self._headers)  # validates the URL
        self._clients = [first]  # every client made, to count connections
        self._idle = [first]
        self._lock = threading.Lock()
        self._last: Optional[ChainState] = None

    @contextlib.contextmanager
    def _client(self) -> Iterator[KeepAliveClient]:
        """Lend an idle pooled client, or a new one when every client is busy."""
        with self._lock:
            client = self._idle.pop() if self._idle else None
        if client is None:
            client = KeepAliveClient(self._base, self._timeout, self._headers)
            with self._lock:
                self._clients.append(client)
        try:
            yield client
        finally:
            with self._lock:
                self._idle.append(client)

    def connections_opened(self) -> int:
        with self._lock:
            return sum(client.connects for client in self._clients)

    def _post(self, client: KeepAliveClient, method: str, params: Any) -> tuple[int, bytes]:
        body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
        try:
            return client.request("POST", self._path, body.encode())
        except (OSError, http.client.HTTPException) as exc:
            raise RequestError(describe(exc)) from exc

    @staticmethod
    def _result(status: int, data: bytes) -> Any:
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

    def call(self, method: str, params: Any) -> Any:
        with self._client() as client:
            status, data = self._post(client, method, params)
        return self._result(status, data)

    def probe(self, t0: float) -> Probe:
        """Fetch the pre-confirmed block and time the round trip.

        The clock runs from when the request went out on an open connection until the whole
        response was read. Opening a connection (a pooled client's first request, or the one
        after an error closed it) is not latency, and neither is decoding the answer: answers
        that arrive together would otherwise wait for each other's decoding.
        """
        with self._client() as client:
            try:
                status, data = self._post(client, self.method, {"block_id": "pre_confirmed"})
            except RequestError as exc:
                recv = time.perf_counter()
                sent = client.sent_at
                return Probe(sent - t0, recv - t0, (recv - sent) * 1000, None, describe(exc))
            recv = time.perf_counter()
            sent = client.sent_at
        timing = (sent - t0, recv - t0, (recv - sent) * 1000)
        try:
            result = self._result(status, data)
            state = ChainState(int(result["block_number"]), tx_hashes(result["transactions"]))
        except (RequestError, KeyError, TypeError, ValueError) as exc:
            return Probe(*timing, None, describe(exc))
        with self._lock:
            if state == self._last:
                state = self._last  # keep one copy of an unchanged answer
            self._last = state
        return Probe(*timing, state)


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


def metrics_url(url: str) -> str:
    """Normalise a --metrics URL: http:// for a bare host:port, /metrics for an empty path."""
    parts = urllib.parse.urlsplit(with_scheme(url))
    if parts.path in ("", "/"):
        parts = parts._replace(path=DEFAULT_METRICS_PATH)
    KeepAliveClient(f"{parts.scheme}://{parts.netloc}", 1.0, {})  # validates, no connection
    return urllib.parse.urlunsplit(parts)


def scrape_gateway_responses(url: str, timeout: float) -> dict[str, int]:
    """Return {HTTP status: count} of get_preconfirmed_block responses from a /metrics page.

    Juno observes feeder_client_request_latency once per attempt that received an HTTP
    response: retries count again, and attempts that failed without a response (timeouts,
    connection errors) are not counted at all.
    """
    parts = urllib.parse.urlsplit(url)
    client = KeepAliveClient(f"{parts.scheme}://{parts.netloc}", timeout, {"Accept": "text/plain"})
    try:
        path = urllib.parse.urlunsplit(("", "", parts.path, parts.query, ""))
        status, data = client.request("GET", path)
    finally:
        client.close()
    if status != 200:
        raise RequestError(f"HTTP {status}")
    counts: dict[str, int] = {}
    for line in data.decode().splitlines():
        match = METRIC_LINE.match(line)
        if not match or match.group(1) != GATEWAY_RESPONSES_METRIC:
            continue
        labels = dict(METRIC_LABEL.findall(match.group(2)))
        if GATEWAY_ENDPOINT not in labels.get("method", ""):
            continue
        status_label = labels.get("status", "?")
        counts[status_label] = counts.get(status_label, 0) + int(float(match.group(3)))
    return counts


def scrape_all(metrics: dict[str, str], timeout: float) -> dict[str, dict[str, int]]:
    result = {}
    for name, url in metrics.items():
        try:
            result[name] = scrape_gateway_responses(url, timeout)
        except (OSError, ValueError, http.client.HTTPException, RequestError) as exc:
            print(f"warning: metrics for {name} unavailable ({describe(exc)})", file=sys.stderr)
    return result


def preflight(seq: SequencerClient, nodes: list[NodeClient]) -> dict[str, str]:
    """Check every endpoint answers and every node is synced; return node versions."""
    try:
        latest = seq.latest_block_number()
    except (OSError, http.client.HTTPException, RequestError) as exc:
        sys.exit(f"sequencer {seq.base_url}: reading the latest block failed: {describe(exc)}")
    sample = seq.poll(time.perf_counter())
    if sample.error:
        sys.exit(f"sequencer {seq.base_url}: {sample.error}")
    if sample.state is None:
        print(
            f"sequencer: latest block {latest}; no pre-confirmed block in the gateway window"
            " right now (HTTP 400)",
            file=sys.stderr,
        )
    else:
        print(
            f"sequencer: latest block {latest}, pre-confirmed block {sample.state.label()}"
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
        # The same comparison Juno's poller makes before it polls (atTip): the node's head
        # against the gateway's latest block. It does not depend on the pre-confirmed window.
        if latest - head > MAX_SYNC_LAG_BLOCKS:
            sys.exit(
                f"{node.name} is {latest - head} blocks behind the gateway's latest block"
                f" {latest}; wait for it to sync"
            )
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


def sample_sequencer(
    seq: SequencerClient,
    t0: float,
    args: argparse.Namespace,
    stop: threading.Event,
    out: queue.SimpleQueue,
) -> None:
    """Read the gateway on a fixed grid of --interval slots anchored at t0.

    A read never starts before its slot, so the script sends at most one request per
    --interval; slots that pass while a read is in flight are skipped, not caught up on.
    """
    slot = 0
    while slot * args.interval < args.duration:
        if stop.wait(max(0.0, t0 + slot * args.interval - time.perf_counter())):
            return
        out.put((SEQUENCER, seq.poll(t0)))
        slot = max(slot + 1, math.floor((time.perf_counter() - t0) / args.interval) + 1)


class NodeSampler:
    """Sends the probe to one node at Poisson arrival times with a mean gap of --interval.

    Random arrivals do not fall into step with a node's own timers (the on-demand node's
    100 ms freshness window, a poller's ticker), so each request sees the node at a random
    phase. The schedule is open loop: each arrival is sent at once, on a thread of its own,
    whether or not the node has answered the earlier ones, so every node is sampled at the
    target rate whatever its latency, and no node's schedule depends on another's. An arrival
    that finds ``cap`` requests still in flight is skipped and counted, which bounds the
    threads and connections a stalled node can hold.
    """

    def __init__(self, node: NodeClient, cap: int, out: queue.SimpleQueue) -> None:
        self.node = node
        self.cap = cap
        self.skipped: list[float] = []  # arrival times (s since run start) not sent at the cap
        self._out = out
        self._lock = threading.Lock()
        self._in_flight: dict[int, float] = {}  # arrival number -> arrival time
        self._closed = False

    def in_flight(self) -> int:
        with self._lock:
            return len(self._in_flight)

    def run(self, t0: float, args: argparse.Namespace, stop: threading.Event) -> None:
        rng = random.Random()
        at = 0.0
        for number in itertools.count():
            at += rng.expovariate(1 / args.interval)
            if at >= args.duration or stop.wait(max(0.0, t0 + at - time.perf_counter())):
                return
            with self._lock:
                if self._closed:
                    return
                if len(self._in_flight) >= self.cap:
                    self.skipped.append(at)
                    continue
                self._in_flight[number] = at
                in_flight = len(self._in_flight)
            threading.Thread(
                target=self._send, args=(number, t0, in_flight), daemon=True
            ).start()

    def _send(self, number: int, t0: float, in_flight: int) -> None:
        probe = None
        try:
            probe = self.node.probe(t0)
            probe.in_flight = in_flight
        finally:
            with self._lock:  # queue the answer before it stops counting as in flight
                if not self._closed:  # once closed, close() has reported it as abandoned
                    del self._in_flight[number]
                    if probe is not None:
                        self._out.put((self.node.name, probe))

    def close(self) -> list[float]:
        """Stop sending and taking answers; return the arrivals still in flight, oldest first."""
        with self._lock:
            self._closed = True
            return sorted(self._in_flight.values())


CSV_COLUMNS = [
    "source", "t_sent_s", "t_recv_s", "latency_ms", "status", "block", "txs", "last_tx",
    "block_identifier", "error",
]
ABANDONED = "abandoned: no answer when the script stopped waiting"


def csv_row(source: str, sample: Any) -> list[Any]:
    """One samples.csv row, for a sequencer read (SequencerSample) or a node probe (Probe)."""
    if isinstance(sample, SequencerSample):
        t_recv, latency, status = sample.t_done, sample.rtt_ms, sample.status
    else:
        t_recv, latency, status = sample.t_recv, sample.latency_ms, ""
    state = sample.state
    return [
        source, f"{sample.t_sent:.4f}", f"{t_recv:.4f}", f"{latency:.1f}", status,
        state.block if state else "", len(state.txs) if state else "",
        hex(state.txs[-1]) if state and state.txs else "", state.identifier if state else "",
        sample.error,
    ]


@dataclass
class NodeStats:
    requests: int = 0  # sent, abandoned ones included
    skipped: int = 0  # arrivals not sent: the node had --max-in-flight requests outstanding
    abandoned: int = 0  # sent, still unanswered when the script stopped waiting
    max_in_flight: int = 0
    errors: int = 0
    latencies: list[float] = field(default_factory=list)
    compared: int = 0
    caught_up: int = 0
    ahead: int = 0
    txs_behind: list[int] = field(default_factory=list)
    block_lag: int = 0
    other_round: int = 0
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
    round_changes: int = 0
    txs_seen: int = 0
    seconds: float = 0.0


class RoundBook:
    """The rounds the script's gateway reads saw, per block, used to place node answers.

    A node's answer has no round identifier, so it is placed by transaction hashes: it is on
    a round when one of the two transaction lists is a prefix of the other. A round's list is
    the longest the script saw over the whole run, so an answer that ran ahead of the
    script's reads still lands on its round. An answer with no transactions fits every
    round of its block; time decides which ones it can be on.
    """

    def __init__(self) -> None:
        self._rounds: dict[int, list[ChainState]] = {}  # per block, in the order first seen
        self._starts: dict[int, list[float]] = {}  # the earliest each of them can have begun
        self._placements: dict[tuple[int, int], tuple[tuple[int, ...], list[int], list[int]]] = {}

    def add(self, state: ChainState, not_before: float) -> int:
        """Record a gateway state; return its round's position among its block's rounds.

        ``not_before`` is when the read before this one was sent: a round first seen now
        cannot have begun earlier.
        """
        rounds = self._rounds.setdefault(state.block, [])
        for order, known in enumerate(rounds):
            if known.identifier == state.identifier:
                if len(state.txs) > len(known.txs):
                    rounds[order] = state
                return order
        rounds.append(state)
        self._starts.setdefault(state.block, []).append(not_before)
        return len(rounds) - 1

    def _place(self, answer: ChainState) -> tuple[list[int], list[int]]:
        """Rounds of the answer's block it fits: sharing transactions, or known only empty."""
        key = (answer.block, id(answer.txs))
        cached = self._placements.get(key)
        if cached is not None and cached[0] is answer.txs:
            return cached[1], cached[2]
        sharing, empty = [], []
        for order, known in enumerate(self._rounds.get(answer.block, [])):
            common = min(len(answer.txs), len(known.txs))
            if answer.txs[:common] == known.txs[:common]:
                (sharing if common else empty).append(order)
        self._placements[key] = (answer.txs, sharing, empty)
        return sharing, empty

    def compare(
        self, answer: ChainState, t_recv: float, state: ChainState, order: int
    ) -> tuple[str, int]:
        """Compare a node's answer, received at ``t_recv``, with a gateway state.

        ``order`` is the state's round. Returns (kind, sign), sign being 1 when the answer is
        newer, 0 equal, -1 older: "block" when the block numbers differ; "round" when the
        answer is on the state's round and the transaction counts decide; "other" when it is
        on another round of the same block: newer if the script saw that round after the
        state's, older if before or never.
        """
        if answer.block != state.block:
            return "block", 1 if answer.block > state.block else -1
        starts = self._starts[state.block]
        if answer.txs:
            sharing, empty = self._place(answer)
            empty = [other for other in empty if starts[other] < t_recv]
            rounds = sharing or empty
            fits = order in sharing or (not sharing and order in empty)
        else:
            # On a newer round with no transactions yet, if one could have begun by then.
            rounds = [other for other in range(order + 1, len(starts)) if starts[other] < t_recv]
            fits = not rounds or not state.txs
        if fits:
            diff = len(answer.txs) - len(state.txs)
            return "round", (diff > 0) - (diff < 0)
        return "other", 1 if rounds and max(rounds) > order else -1


def evaluate(
    seq_samples: list[SequencerSample],
    probes: dict[str, list[Probe]],
    window: tuple[float, float],
    skipped: Optional[dict[str, list[float]]] = None,
    abandoned: Optional[dict[str, list[float]]] = None,
) -> tuple[dict[str, NodeStats], SequencerStats]:
    """Compute per-node and sequencer statistics for requests sent inside the window.

    Samples before the window (warm-up) still seed the sequencer timeline, and node answers
    from any time count as sightings of a new state. A node's requests overlap, so its
    answers may come in any order: a later request can be answered first. ``skipped`` and
    ``abandoned`` hold, per node, the arrival times not sent at the in-flight cap and those
    of requests still unanswered when the script stopped waiting.
    """
    start, end = window
    names = list(probes)
    stats = {name: NodeStats() for name in names}
    seq_stats = SequencerStats(seconds=max(0.0, end - start))
    book = RoundBook()
    orders = [
        book.add(s.state, seq_samples[i - 1].t_sent if i else -math.inf) if s.state else -1
        for i, s in enumerate(seq_samples)
    ]

    for sample in seq_samples:
        if sample.t_sent < start:
            continue
        seq_stats.samples += 1
        if sample.error:
            seq_stats.errors += 1
        elif sample.state is None:
            seq_stats.no_window += 1
        if sample.status == 200:
            seq_stats.rtts.append(sample.rtt_ms)

    # Each answer against the newest state the gateway reads had shown when it was sent.
    done_times = [s.t_done for s in seq_samples]
    for name in names:
        node = stats[name]
        node.skipped = sum(1 for at in (skipped or {}).get(name, ()) if at >= start)
        node.abandoned = sum(1 for at in (abandoned or {}).get(name, ()) if at >= start)
        node.requests = node.abandoned
        for probe in probes[name]:
            if probe.t_sent < start:
                continue
            node.requests += 1
            node.max_in_flight = max(node.max_in_flight, probe.in_flight)
            if probe.state is None:
                node.errors += 1
                continue
            node.latencies.append(probe.latency_ms)
            ref = bisect.bisect_right(done_times, probe.t_sent) - 1
            if ref < 0 or seq_samples[ref].state is None:
                continue  # that read failed: nothing current to compare with
            state = seq_samples[ref].state
            kind, sign = book.compare(probe.state, probe.t_recv, state, orders[ref])
            node.compared += 1
            if sign >= 0:
                node.caught_up += 1
            if sign > 0:
                node.ahead += 1
            if kind == "round":
                node.txs_behind.append(max(0, len(state.txs) - len(probe.state.txs)))
            elif sign < 0 and kind == "block":
                node.block_lag += 1
            elif sign < 0:
                node.other_round += 1

    # New sequencer states: a block roll, a new round at the same height, or more transactions
    # in the same round. States seen during warm-up only seed the comparison. A node's
    # answers go by receive time: the first to show a state is when the client had it.
    answered = {
        name: sorted((p for p in probes[name] if p.state is not None), key=lambda p: p.t_recv)
        for name in names
    }
    recv_times = {name: [p.t_recv for p in answered[name]] for name in names}
    best: Optional[tuple[int, int, int]] = None
    for i, sample in enumerate(seq_samples):
        state = sample.state
        if state is None:
            continue
        rank = (state.block, orders[i], len(state.txs))
        if best is not None and rank <= best:
            continue
        previous, best = best, rank
        if previous is None or sample.t_sent < start:
            continue
        seq_stats.events += 1
        if state.block != previous[0]:
            seq_stats.block_rolls += 1
            seq_stats.txs_seen += len(state.txs)
        elif orders[i] != previous[1]:
            seq_stats.round_changes += 1
            seq_stats.txs_seen += len(state.txs)
        else:
            seq_stats.txs_seen += len(state.txs) - previous[2]
        # The read before this one did not show the state, so no answer received before it
        # was sent can reflect it: sightings start there, one read interval early at most.
        not_before = seq_samples[i - 1].t_sent
        first_seen = {}
        for name in names:
            start_at = bisect.bisect_right(recv_times[name], not_before)
            for probe in itertools.islice(answered[name], start_at, None):
                if book.compare(probe.state, probe.t_recv, state, orders[i])[1] >= 0:
                    first_seen[name] = probe.t_recv
                    stats[name].time_to_see_ms.append((probe.t_recv - sample.t_done) * 1000)
                    break
            else:
                stats[name].unseen += 1
        if first_seen:
            earliest = min(first_seen.values())
            for name, t_recv in first_seen.items():
                if (t_recv - earliest) * 1000 <= SAME_TIME_MS:
                    stats[name].saw_first += 1
    return stats, seq_stats


def percentile(values: list[float], pct: float) -> float:
    ordered = sorted(values)
    rank = max(1, math.ceil(pct / 100 * len(ordered)))
    return ordered[rank - 1]


def fmt_ms(value: float) -> str:
    return f"{value:.1f}" if abs(value) < 10 else f"{value:.0f}"


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
    connections: Optional[dict[str, int]] = None,
) -> str:
    seconds = seq_stats.seconds

    def per_s(count: int) -> str:
        return f"{count / seconds:.1f}" if seconds else "n/a"

    lines = ["# Pre-confirmed freshness benchmark", ""]
    lines.append(
        f"- started {started_at.strftime('%Y-%m-%dT%H:%M:%SZ')}; measured {seconds:.0f} s after"
        f" a {args.warmup:g} s warm-up; probe `{args.method}`"
    )
    lines.append(
        f"- sampling: the gateway on a fixed {args.interval:g} s grid ({per_s(seq_stats.samples)}"
        f" reads/s, asked for {1 / args.interval:.1f}/s); each node on its own Poisson schedule"
        f" with a mean gap of {args.interval:g} s, every arrival sent whether or not the node"
        f" had answered the earlier ones, up to {args.max_in_flight} requests in flight per node"
        " (arrivals beyond that skipped); reqs/s below is the rate achieved"
    )
    lines.append(
        f"- sequencer `{seq.base_url}`: {seq_stats.samples} reads, {seq_stats.errors} errors,"
        f" {seq_stats.no_window} without a pre-confirmed block (HTTP 400); {seq_stats.events} new"
        f" states ({seq_stats.block_rolls} block rolls, {seq_stats.round_changes} same-height"
        f" round replacements, {per_s(seq_stats.txs_seen)} tx/s observed);"
        f" rtt p50/p90 {fmt_pcts(seq_stats.rtts, (50, 90))} ms"
    )
    lines.append(
        f"- skipped: arrivals not sent because {args.max_in_flight} requests to the node were"
        " still in flight. max in flight: the most requests to the node outstanding at once."
        " lat: from sending the request on an open connection to reading the whole answer."
        " caught up / ahead: the node's answer was at least / newer than the newest state the"
        " script's gateway reads had shown when the request was sent (not compared when that"
        " read failed). txs behind: same block and round. block lag: an older block. other"
        " round: the same block on a round the gateway had replaced, or one it never showed."
        " time to see: from the script's first gateway read showing a new state until the"
        " client received it (or newer) from the node, whichever request brought it first;"
        " request latency and the wait for the node's next request included; negative when"
        " the node returned it before that read (answers received before the previous read was"
        " sent don't count). saw first:"
        " share of new states this node returned before every other node. gw resp: HTTP"
        " responses the node's feeder client got from get_preconfirmed_block, one per attempt;"
        " attempts that got no response (timeouts, connection errors) are not counted."
    )
    lines.append("")
    header = (
        "| node | version | reqs | reqs/s | skipped | max in flight | errors | lat avg (ms) | p50"
        " | p90 | p99 | max | gw resp | gw resp/req | gw resp/s | gw non-200 |"
    )
    lines.append(header)
    lines.append("|" + " --- |" * (header.count("|") - 1))
    responses: dict[str, dict[str, int]] = {}
    for name in names:
        node = stats[name]
        lat = node.latencies
        if name in gw_start and name in gw_end:
            calls = {
                status: gw_end[name].get(status, 0) - gw_start[name].get(status, 0)
                for status in set(gw_start[name]) | set(gw_end[name])
            }
            responses[name] = {status: count for status, count in calls.items() if count}
            total = sum(calls.values())
            bad = sum(count for status, count in calls.items() if status != "200")
            per_request = f"{total / node.requests:.2f}" if node.requests else "n/a"
            gw_cells = f"{total} | {per_request} | {per_s(total)} | {bad}"
        else:
            gw_cells = "n/a | n/a | n/a | n/a"
        lines.append(
            f"| {name} | {versions.get(name, '?')} | {node.requests} | {per_s(node.requests)}"
            f" | {node.skipped} | {node.max_in_flight} | {node.errors}"
            f" | {fmt_ms(statistics.fmean(lat)) if lat else 'n/a'}"
            f" | {fmt_pcts(lat, (50,))} | {fmt_pcts(lat, (90,))} | {fmt_pcts(lat, (99,))}"
            f" | {fmt_ms(max(lat)) if lat else 'n/a'} | {gw_cells} |"
        )
    lines.append("")
    abandoned = [f"{name} {stats[name].abandoned}" for name in names if stats[name].abandoned]
    if abandoned:
        lines.append(
            f"Requests still unanswered when the script stopped waiting: {', '.join(abandoned)};"
            " counted in reqs but not in errors or latency (rows with the error `abandoned` in"
            " samples.csv)."
        )
    if connections:
        lines.append(
            "Connections opened over the run (one per request in flight, reused; an error"
            " closes one): "
            + ", ".join(f"{name} {connections[name]}" for name in names if name in connections)
            + "."
        )
    if abandoned or connections:
        lines.append("")
    if responses:
        breakdown = "; ".join(
            f"{name} "
            + (" ".join(f"{status}={count}" for status, count in sorted(counts.items())) or "none")
            for name, counts in responses.items()
        )
        lines.append(f"Gateway responses by HTTP status: {breakdown}.")
        silent = [name for name, counts in responses.items() if not counts and stats[name].requests]
        if silent:
            lines.append(
                f"No gateway responses at all for {', '.join(silent)}: polls that fail without"
                " an HTTP response (timeouts, connection errors) leave no trace in /metrics."
            )
        lines.append("")
    header = (
        "| node | compared | caught up | ahead | txs behind p50/p90/max | block lag | other round"
        " | time to see p50/p90/max (ms) | unseen | saw first |"
    )
    lines.append(header)
    lines.append("|" + " --- |" * (header.count("|") - 1))
    for name in names:
        node = stats[name]
        lines.append(
            f"| {name} | {node.compared} | {fmt_share(node.caught_up, node.compared)}"
            f" | {fmt_share(node.ahead, node.compared)}"
            f" | {fmt_pcts([float(v) for v in node.txs_behind])} | {node.block_lag}"
            f" | {node.other_round} | {fmt_pcts(node.time_to_see_ms)} | {node.unseen}"
            f" | {fmt_share(node.saw_first, seq_stats.events)} |"
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
    if SEQUENCER in node_urls:
        sys.exit(f"--node name {SEQUENCER!r} is reserved for the gateway reads")
    try:
        nodes = [
            NodeClient(name, url, args.method, args.timeout) for name, url in node_urls.items()
        ]
        metrics = {name: metrics_url(url) for name, url in metrics.items()}
        seq = SequencerClient(args.sequencer, args.api_key, args.timeout)
    except ValueError as exc:
        sys.exit(str(exc))
    names = [node.name for node in nodes]
    versions = preflight(seq, nodes)

    started_at = datetime.now(timezone.utc)
    stamp = started_at.strftime("%Y%m%dT%H%M%SZ")
    out_dir = Path(args.out) if args.out else Path(__file__).resolve().parent / "results" / stamp
    out_dir.mkdir(parents=True, exist_ok=True)
    seq_samples: list[SequencerSample] = []
    probes: dict[str, list[Probe]] = {name: [] for name in names}
    samples: queue.SimpleQueue = queue.SimpleQueue()
    stop = threading.Event()
    gw_start: dict[str, dict[str, int]] = {}
    gw_end: dict[str, dict[str, int]] = {}
    measuring = False
    end = args.duration
    samplers = [NodeSampler(node, args.max_in_flight, samples) for node in nodes]
    abandoned: dict[str, list[float]] = {}
    interrupts: list[float] = []  # perf_counter() of each Ctrl-C
    t0 = time.perf_counter()
    threads = [
        threading.Thread(target=sample_sequencer, args=(seq, t0, args, stop, samples), daemon=True)
    ]
    threads += [
        threading.Thread(target=sampler.run, args=(t0, args, stop), daemon=True)
        for sampler in samplers
    ]
    next_progress = PROGRESS_EVERY_S
    print(f"running for {args.duration:g} s; results in {out_dir}", file=sys.stderr)
    # Ctrl-C ends the run and a second one the wait for answers in flight; either way the
    # report is written. Handling the signal instead of KeyboardInterrupt keeps a Ctrl-C from
    # landing in the middle of a CSV write or a metrics scrape.
    previous_handler = signal.getsignal(signal.SIGINT)
    handle_sigint = previous_handler not in (signal.SIG_IGN, None)  # None: not set from Python
    if handle_sigint:
        signal.signal(signal.SIGINT, lambda signum, frame: interrupts.append(time.perf_counter()))
    try:
        with (out_dir / "samples.csv").open("w", newline="") as fh:
            writer = csv.writer(fh)
            writer.writerow(CSV_COLUMNS)

            def drain() -> None:
                while True:
                    try:
                        source, sample = samples.get_nowait()
                    except queue.Empty:
                        fh.flush()
                        return
                    (seq_samples if source == SEQUENCER else probes[source]).append(sample)
                    writer.writerow(csv_row(source, sample))

            for thread in threads:
                thread.start()
            while not interrupts:
                t = time.perf_counter() - t0
                if t >= args.duration:
                    break
                if not measuring and t >= args.warmup:
                    measuring = True
                    gw_start = scrape_all(metrics, args.timeout)
                drain()
                if t >= next_progress:
                    next_progress += PROGRESS_EVERY_S
                    last = seq_samples[-1] if seq_samples else None
                    seq_seen = (
                        "?" if last is None
                        else last.state.label() if last.state
                        else (last.error or "no window")
                    )
                    seen = ", ".join(
                        f"{name}={p[-1].state.label() if p[-1].state else p[-1].error}"
                        for name, p in probes.items() if p
                    )
                    skipped = ", ".join(
                        f"{sampler.node.name}={len(sampler.skipped)}"
                        for sampler in samplers if sampler.skipped
                    )
                    print(
                        f"t={t:6.1f}s sequencer={seq_seen} {seen}"
                        + (f"; skipped at the in-flight cap: {skipped}" if skipped else ""),
                        file=sys.stderr,
                    )
                time.sleep(min(0.05, max(0.0, args.duration - (time.perf_counter() - t0))))
            stop.set()
            if interrupts:
                end = min(end, time.perf_counter() - t0)  # sending stopped here
                print("interrupted; writing the report", file=sys.stderr)
            if measuring:  # the window ends here, not when the last answer comes in
                gw_end = scrape_all(metrics, args.timeout)

            # Requests sent before the end still count. Their answers are due within
            # --timeout; wait that long and a second more, unless Ctrl-C says not to.
            wait_until = time.perf_counter() + args.timeout + 1
            seen_interrupts = len(interrupts)
            told = False
            while threads[0].is_alive() or any(sampler.in_flight() for sampler in samplers):
                now = time.perf_counter()
                if now >= wait_until or len(interrupts) > seen_interrupts:
                    break
                if not told and (interrupts or now - t0 >= end + 1):
                    pending = sum(sampler.in_flight() for sampler in samplers)
                    print(
                        f"waiting up to {wait_until - now:.0f} s for {pending} requests in"
                        " flight; Ctrl-C to stop waiting",
                        file=sys.stderr,
                    )
                    told = True
                drain()
                time.sleep(0.02)
            abandoned = {sampler.node.name: sampler.close() for sampler in samplers}
            drain()
            for name, arrivals in abandoned.items():
                for at in arrivals:
                    writer.writerow([name, f"{at:.4f}", "", "", "", "", "", "", "", ABANDONED])
            if any(abandoned.values()):
                print(
                    f"stopped waiting; {sum(map(len, abandoned.values()))} requests abandoned",
                    file=sys.stderr,
                )

        stats, seq_stats = evaluate(
            seq_samples,
            probes,
            (args.warmup, end),
            {sampler.node.name: sampler.skipped for sampler in samplers},
            abandoned,
        )
        connections = {node.name: node.connections_opened() for node in nodes}
        report = build_report(
            args, started_at, names, versions, seq, stats, seq_stats, gw_start, gw_end, connections
        )
        (out_dir / "report.md").write_text(report)
    finally:
        if handle_sigint:
            signal.signal(signal.SIGINT, previous_handler)
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
        help="Juno JSON-RPC endpoint; repeatable. A bare host:port means http://host:port/v0_10.",
    )
    parser.add_argument(
        "--metrics", action="append", default=[], metavar="NAME=URL",
        help="Juno Prometheus /metrics URL for a node named by --node; repeatable, optional."
        " A bare host:port means http://host:port/metrics.",
    )
    parser.add_argument("--sequencer", default=DEFAULT_SEQUENCER, help="feeder gateway base URL")
    parser.add_argument("--api-key", default="", help="X-Throttling-Bypass for sequencer calls")
    parser.add_argument(
        "--method", default="starknet_getBlockWithTxHashes",
        choices=["starknet_getBlockWithTxHashes", "starknet_getBlockWithTxs"],
        help="JSON-RPC method used to read the pre-confirmed block",
    )
    parser.add_argument(
        "--interval", type=float, default=0.2,
        help="seconds between the script's gateway reads, and the mean gap between requests to"
        " each node",
    )
    parser.add_argument("--duration", type=float, default=300.0, help="total run time in seconds")
    parser.add_argument("--warmup", type=float, default=15.0, help="seconds excluded from stats")
    parser.add_argument("--timeout", type=float, default=5.0, help="per-request timeout, seconds")
    parser.add_argument(
        "--max-in-flight", type=int, default=0, metavar="N",
        help="requests in flight per node at most; an arrival that finds N outstanding is skipped"
        " and counted. 0 means --timeout / --interval rounded up, at least 4: as many as arrive,"
        " on average, while a request runs into the timeout (25 with the defaults)",
    )
    parser.add_argument(
        "--out", default="",
        help="output directory (default: results/<timestamp> in the script's directory)",
    )
    args = parser.parse_args(argv)
    if args.interval <= 0 or args.duration <= 0 or args.timeout <= 0 or args.warmup < 0:
        parser.error("--interval, --duration and --timeout must be positive; --warmup >= 0")
    if args.warmup >= args.duration:
        parser.error("--warmup must be shorter than --duration")
    if args.max_in_flight < 0:
        parser.error("--max-in-flight must be positive, or 0 for the default")
    if not args.max_in_flight:
        # The epsilon keeps float noise (0.9 / 0.03 = 30.000000000000004) from adding one.
        args.max_in_flight = max(4, math.ceil(args.timeout / args.interval - 1e-9))
    return args


if __name__ == "__main__":
    sys.exit(run(parse_args()))
