"use strict";

const allEndpoints = "__all__";
const endpointSelect = document.getElementById("endpoint");
const windowSelect = document.getElementById("window");
const statusElement = document.getElementById("status");
const summaryElement = document.getElementById("summary");
const configurationElement = document.getElementById("configuration");
const overviewCharts = document.getElementById("overview-charts");
const latencyPanel = document.getElementById("latency-panel");
const throughputPanel = document.getElementById("throughput-panel");
const summaryPathPattern = /^runs\/[A-Za-z0-9][A-Za-z0-9._-]*\/summary\.json$/;
const latencySeries = [
  {label: "p50", metric: "med", color: "#67a3d9"},
  {label: "p95", metric: "p95", color: "#2855d9"},
  {label: "p99", metric: "p99", color: "#8e44ad"}
];
let runs = [];
let endpoints = [];
let charts = [];
let skippedRuns = 0;

async function fetchJson(path) {
  const response = await fetch(path, {cache: "no-store"});
  if (!response.ok) {
    throw new Error(`${path}: HTTP ${response.status}`);
  }
  return response.json();
}

function cutoffDate() {
  const cutoff = new Date();
  cutoff.setMonth(cutoff.getMonth() - Number(windowSelect.value));
  return cutoff;
}

function validHistoryEntry(entry, cutoff) {
  return entry &&
    typeof entry.startedAt === "string" &&
    !Number.isNaN(Date.parse(entry.startedAt)) &&
    new Date(entry.startedAt) >= cutoff &&
    typeof entry.summary === "string" &&
    summaryPathPattern.test(entry.summary);
}

function validSummary(summary, runId) {
  return summary &&
    summary.schemaVersion === 1 &&
    summary.run &&
    summary.run.id === runId &&
    summary.juno &&
    Array.isArray(summary.cases);
}

function option(value, label) {
  const element = document.createElement("option");
  element.value = value;
  element.textContent = label;
  return element;
}

async function load() {
  const selectedEndpoint = endpointSelect.value || allEndpoints;
  statusElement.textContent = "Loading results...";
  const history = await fetchJson("history.json");
  if (history.schemaVersion !== 1 || !Array.isArray(history.runs)) {
    throw new Error("Unsupported history format");
  }

  const entries = history.runs.filter((entry) =>
    validHistoryEntry(entry, cutoffDate())
  );
  const loadedRuns = await Promise.all(entries.map(async (entry) => {
    try {
      const summary = await fetchJson(entry.summary);
      if (!validSummary(summary, entry.id)) {
        throw new Error("unsupported summary format");
      }
      return {entry, summary};
    } catch (error) {
      console.warn(`Skipping ${entry.summary}: ${error.message}`);
      return null;
    }
  }));
  skippedRuns = loadedRuns.filter((run) => run === null).length;
  runs = loadedRuns.filter((run) => run !== null);
  runs.sort((a, b) => new Date(a.entry.startedAt) - new Date(b.entry.startedAt));
  endpoints = [...new Set(runs.flatMap(({summary}) =>
    summary.cases.map((item) => item.id)
  ))].filter(Boolean).sort();

  endpointSelect.replaceChildren(
    option(allEndpoints, "All endpoints"),
    ...endpoints.map((id) => option(id, id))
  );
  endpointSelect.value = endpoints.includes(selectedEndpoint)
    ? selectedEndpoint
    : allEndpoints;
  render();
}

function revisionLabel(version) {
  const describedCommit = String(version || "").match(/-g([0-9a-f]{7,40})$/i);
  if (describedCommit) {
    return describedCommit[1].slice(0, 7);
  }
  if (version && String(version).startsWith("v")) {
    return String(version);
  }
  const value = String(version || "unknown").replace(/^sha-/i, "");
  return /^[0-9a-f]{8,40}$/i.test(value) ? value.slice(0, 7) : value;
}

function formatNumber(value, digits = 2) {
  return typeof value === "number" ? value.toFixed(digits) : "n/a";
}

function addCard(label, value, state) {
  const card = document.createElement("div");
  const strong = document.createElement("strong");
  card.className = "card";
  strong.textContent = value;
  if (state === "passed" || state === "failed") {
    strong.className = state;
  }
  card.append(document.createTextNode(label), strong);
  summaryElement.append(card);
}

function addConfiguration(label, value) {
  const item = document.createElement("div");
  const term = document.createElement("dt");
  const description = document.createElement("dd");
  term.textContent = label;
  description.textContent = value ?? "n/a";
  item.append(term, description);
  configurationElement.append(item);
}

function resourceSummary(resources) {
  if (!resources?.requests || !resources?.limits) {
    return "n/a";
  }
  const requests = `${resources.requests.cpu} CPU · ${resources.requests.memory}`;
  const limits = `${resources.limits.cpu} CPU · ${resources.limits.memory}`;
  return requests === limits ? requests : `requests ${requests} · limits ${limits}`;
}

function dateCounts(points) {
  const counts = new Map();
  for (const point of points) {
    const date = point.entry.startedAt.slice(0, 10);
    counts.set(date, (counts.get(date) || 0) + 1);
  }
  return counts;
}

function pointLabel(point, counts) {
  const date = point.entry.startedAt.slice(0, 10);
  const time = point.entry.startedAt.slice(11, 16);
  const timestamp = counts.get(date) > 1 ? `${date} ${time}` : date;
  return `${timestamp} · ${revisionLabel(point.entry.junoVersion)}`;
}

function pointsFor(endpoint) {
  return runs
    .map((run) => ({
      ...run,
      value: run.summary.cases.find((item) => item.id === endpoint)
    }))
    .filter(({value}) => value);
}

function statusColors(values, color) {
  return values.map((value) =>
    value.status === "failed" ? "#b42318" : color
  );
}

function chartDataset(label, data, color, colors = color, showLine = true) {
  return {
    label,
    data,
    borderColor: color,
    backgroundColor: color,
    pointBackgroundColor: colors,
    pointRadius: 5,
    pointHoverRadius: 7,
    showLine,
    cubicInterpolationMode: "monotone"
  };
}

function latencyDatasets(values, pointColor, showLine = true) {
  return latencySeries.map(({label, metric, color}) => chartDataset(
    label,
    values.map((value) => value.latencyMs?.[metric]),
    color,
    statusColors(values, pointColor ?? color),
    showLine
  ));
}

function chartOptions(title, logarithmic = false, discrete = false) {
  const scales = {
    y: logarithmic
      ? {type: "logarithmic", beginAtZero: false}
      : {beginAtZero: true}
  };
  if (discrete) {
    scales.x = {
      ticks: {autoSkip: false, minRotation: 45, maxRotation: 60}
    };
  }
  return {
    maintainAspectRatio: false,
    interaction: {intersect: false, mode: "index"},
    plugins: {title: {display: true, text: title}},
    scales
  };
}

function createLatencyChart(canvas, endpoint, points) {
  const counts = dateCounts(points);
  const labels = points.map((point) => pointLabel(point, counts));
  const values = points.map(({value}) => value);
  charts.push(new Chart(canvas, {
    type: "line",
    data: {
      labels,
      datasets: latencyDatasets(values, "#2855d9")
    },
    options: chartOptions(`${endpoint} latency (ms)`)
  }));
}

function createThroughputChart(canvas, endpoint, points) {
  const counts = dateCounts(points);
  const values = points.map(({value}) => value);
  charts.push(new Chart(canvas, {
    type: "line",
    data: {
      labels: points.map((point) => pointLabel(point, counts)),
      datasets: [chartDataset(
        "Requests/second",
        values.map((value) => value.requests?.rate),
        "#08783e",
        statusColors(values, "#2855d9")
      )]
    },
    options: chartOptions(`${endpoint} throughput`)
  }));
}

function populateLatest(latest, endpoint) {
  summaryElement.replaceChildren();
  configurationElement.replaceChildren();
  if (!latest) {
    addCard("Results", "No runs in this window");
    addConfiguration("Configuration", "No run selected");
    return;
  }

  const load = latest.summary.load
    ? `${latest.summary.load.vus} VUs · ${latest.summary.load.duration}`
    : "n/a";
  addCard(
    "Juno version",
    latest.entry.junoVersion || latest.summary.juno.version || "unknown"
  );
  addCard("Overall run", latest.summary.run.status, latest.summary.run.status);
  if (endpoint) {
    addCard("Endpoint status", latest.value.status, latest.value.status);
    addCard("Latest p95", `${formatNumber(latest.value.latencyMs?.p95)} ms`);
    addCard("Requests/second", formatNumber(latest.value.requests?.rate));
    addCard(
      "Request failure rate",
      typeof latest.value.requests?.failureRate === "number"
        ? `${formatNumber(latest.value.requests.failureRate * 100)}%`
        : "n/a"
    );
    addCard(
      "RPC result check failures",
      String(latest.value.checks?.failures ?? "n/a")
    );
  } else {
    const passed = latest.summary.cases.filter((item) => item.status === "passed").length;
    const failed = latest.summary.cases.length - passed;
    addCard("Endpoints", String(latest.summary.cases.length));
    addCard("Passed endpoints", String(passed), failed === 0 ? "passed" : undefined);
    addCard("Failed endpoints", String(failed), failed === 0 ? "passed" : "failed");
  }
  addCard("Load", load);
  addConfiguration("Run ID", latest.summary.run.id);
  addConfiguration("Started at", latest.summary.run.startedAt);
  addConfiguration("Load profile", load);
  addConfiguration("RPC version", latest.summary.juno.rpcVersion);
  addConfiguration(
    "Snapshot",
    latest.summary.snapshot
      ? `${latest.summary.snapshot.id} · block ${latest.summary.juno.blockNumber}`
      : "n/a"
  );
  addConfiguration("Snapshot SHA-256", latest.summary.snapshot?.sha256);
  addConfiguration("Juno image digest", latest.summary.juno.imageDigest);
  addConfiguration("Benchmark image digest", latest.summary.benchmark?.imageDigest);
  addConfiguration(
    "Juno resources",
    resourceSummary(latest.summary.runtime?.juno?.resources)
  );
  addConfiguration(
    "Benchmark runner resources",
    resourceSummary(latest.summary.runtime?.benchmarkRunner?.resources)
  );
  addConfiguration(
    "Juno arguments",
    latest.summary.runtime?.juno?.args?.join(" ") || "n/a"
  );
}

function destroyCharts() {
  for (const chart of charts) {
    chart.destroy();
  }
  charts = [];
  overviewCharts.replaceChildren();
}

function dailySection(run, counts) {
  const section = document.createElement("section");
  const heading = document.createElement("h2");
  const chart = document.createElement("div");
  const canvas = document.createElement("canvas");
  heading.textContent = pointLabel(run, counts);
  chart.className = "daily-chart";
  chart.append(canvas);
  section.append(heading, chart);
  overviewCharts.append(section);
  return canvas;
}

function createDailyChart(canvas, run) {
  const values = run.summary.cases;
  const labels = values.map((value) => value.id);
  charts.push(new Chart(canvas, {
    type: "line",
    data: {
      labels,
      datasets: latencyDatasets(values, null, false)
    },
    options: chartOptions("Endpoint latency (ms)", true, true)
  }));
}

function renderOverview() {
  populateLatest(runs.at(-1));
  statusElement.textContent = `${runs.length} run(s) · ${endpoints.length} endpoints · ` +
    `${windowSelect.value}-month window${skippedLabel()}`;
  overviewCharts.hidden = false;
  latencyPanel.hidden = true;
  throughputPanel.hidden = true;

  const counts = dateCounts(runs);
  for (const run of [...runs].reverse()) {
    createDailyChart(dailySection(run, counts), run);
  }
}

function renderEndpoint(endpoint) {
  const points = pointsFor(endpoint);
  populateLatest(points.at(-1), endpoint);
  statusElement.textContent = `${points.length} run(s) · ${endpoint} · ` +
    `${windowSelect.value}-month window${skippedLabel()}`;
  overviewCharts.hidden = true;
  latencyPanel.hidden = false;
  throughputPanel.hidden = false;
  createLatencyChart(document.getElementById("latency"), endpoint, points);
  createThroughputChart(document.getElementById("throughput"), endpoint, points);
}

function skippedLabel() {
  return skippedRuns === 0 ? "" : ` · ${skippedRuns} invalid run(s) skipped`;
}

function render() {
  destroyCharts();
  if (endpointSelect.value === allEndpoints) {
    renderOverview();
  } else {
    renderEndpoint(endpointSelect.value);
  }
}

function reload() {
  load().catch((error) => {
    statusElement.textContent = `Could not load results: ${error.message}`;
  });
}

endpointSelect.addEventListener("change", render);
windowSelect.addEventListener("change", reload);
reload();
