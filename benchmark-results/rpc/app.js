"use strict";

const allEndpoints = "__all__";
const endpointSelect = document.getElementById("endpoint");
const windowSelect = document.getElementById("window");
const viewSelect = document.getElementById("view");
const runCountSelect = document.getElementById("run-count");
const runCountControl = document.getElementById("run-count-control");
const statusElement = document.getElementById("status");
const summaryElement = document.getElementById("summary");
const configurationElement = document.getElementById("configuration");
const overviewCharts = document.getElementById("overview-charts");
const highlightsElement = document.getElementById("highlights");
const latencyPanel = document.getElementById("latency-panel");
const throughputPanel = document.getElementById("throughput-panel");
const summaryPathPattern = /^runs\/[A-Za-z0-9][A-Za-z0-9._-]*\/summary\.json$/;
const latencySeries = [
  {label: "p50", metric: "med", color: "#8e44ad"},
  {label: "p95", metric: "p95", color: "#2855d9"},
  {label: "p99", metric: "p99", color: "#67a3d9"}
];
const percentileSeries = [
  {label: "p50", metric: "med"},
  {label: "p90", metric: "p90"},
  {label: "p95", metric: "p95"},
  {label: "p99", metric: "p99"},
  {label: "max", metric: "max"}
];
// Ordinal blue ramp, oldest run lightest. Sampling it for more than seven runs
// puts adjacent runs below a distinguishable lightness gap, which is what caps
// the run-count control.
const recencyRamp = [
  "#86b6ef", "#6da7ec", "#5598e7", "#3987e5", "#2a78d6",
  "#256abf", "#1c5cab", "#184f95", "#104281", "#0d366b"
];
const highlightMetrics = [
  {label: "p50", read: (item) => item.latencyMs?.med, worseWhen: "higher"},
  {label: "p95", read: (item) => item.latencyMs?.p95, worseWhen: "higher"},
  {label: "throughput", read: (item) => item.requests?.rate, worseWhen: "lower"}
];
// A case must clear both gates to be called a mover: the score keeps methods
// with a wide spread of their own from crying wolf, and the change floor keeps a
// small wobble on a very steady method from reading as a regression.
const minBaselineRuns = 5;
const minScore = 3;
const minChange = 0.1;
// Runs of the same case land within a few percent of each other at best, so a
// narrower spread than this is precision the harness does not have.
const scatterFloor = 0.04;
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
  strong.title = value;
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

function mixHex(from, to, ratio) {
  const channel = (offset) => {
    const start = parseInt(from.slice(offset, offset + 2), 16);
    const end = parseInt(to.slice(offset, offset + 2), 16);
    return Math.round(start + (end - start) * ratio)
      .toString(16)
      .padStart(2, "0");
  };
  return `#${channel(1)}${channel(3)}${channel(5)}`;
}

function recencyColor(index, total) {
  const last = recencyRamp.length - 1;
  const position = total < 2 ? last : (index / (total - 1)) * last;
  const step = Math.floor(position);
  return mixHex(
    recencyRamp[step],
    recencyRamp[Math.min(step + 1, last)],
    position - step
  );
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

function latencyDatasets(values, showLine = true) {
  return latencySeries.map(({label, metric, color}) => chartDataset(
    label,
    values.map((value) => value.latencyMs?.[metric]),
    color,
    statusColors(values, color),
    showLine
  ));
}

// Common to every chart here: fill the container, and hover the whole x slice
// rather than one mark. The two builders below add what their shape needs.
function baseOptions(title) {
  return {
    maintainAspectRatio: false,
    interaction: {intersect: false, mode: "index"},
    plugins: {title: {display: true, text: title}}
  };
}

// Full-width charts.
function chartOptions(title, logarithmic = false, discrete = false) {
  const options = baseOptions(title);
  options.scales = {
    y: logarithmic
      ? {type: "logarithmic", beginAtZero: false}
      : {beginAtZero: true}
  };
  if (discrete) {
    options.scales.x = {
      ticks: {autoSkip: false, minRotation: 45, maxRotation: 60}
    };
  }
  return options;
}

function createLatencyChart(canvas, endpoint, points) {
  const counts = dateCounts(points);
  const labels = points.map((point) => pointLabel(point, counts));
  const values = points.map(({value}) => value);
  charts.push(new Chart(canvas, {
    type: "line",
    data: {
      labels,
      datasets: latencyDatasets(values)
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
        statusColors(values, "#08783e")
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
    addCard("Requests/s", formatNumber(latest.value.requests?.rate));
    addCard(
      "Failure rate",
      typeof latest.value.requests?.failureRate === "number"
        ? `${formatNumber(latest.value.requests.failureRate * 100)}%`
        : "n/a"
    );
    addCard(
      "Check failures",
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
  addConfiguration(
    "Juno version",
    latest.entry.junoVersion || latest.summary.juno.version
  );
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

// One legend above the grid; per-panel legends would repeat it 23 times.
function legendList(entries) {
  const legend = document.createElement("ul");
  legend.className = "chart-legend";
  for (const entry of entries) {
    const item = document.createElement("li");
    const swatch = document.createElement("span");
    swatch.className = "swatch";
    swatch.style.background = entry.color;
    item.append(swatch, document.createTextNode(entry.label));
    legend.append(item);
  }
  return legend;
}

function chartGrid() {
  const grid = document.createElement("div");
  grid.className = "chart-grid";
  return grid;
}

function gridPanel(grid) {
  const panel = document.createElement("div");
  const chart = document.createElement("div");
  const canvas = document.createElement("canvas");
  panel.className = "chart-panel";
  chart.className = "panel-chart";
  chart.append(canvas);
  panel.append(chart);
  grid.append(panel);
  return canvas;
}

// Grid panels: no per-panel legend, since one legend sits above the grid.
function panelOptions(endpoint, xTicks, tooltipTitles) {
  const options = baseOptions(endpoint);
  options.plugins.legend = {display: false};
  if (tooltipTitles) {
    options.plugins.tooltip = {
      callbacks: {title: (items) => tooltipTitles[items[0].dataIndex]}
    };
  }
  options.scales = {
    x: {ticks: xTicks},
    y: {
      type: "logarithmic",
      beginAtZero: false,
      title: {display: true, text: "ms"}
    }
  };
  return options;
}

// One dataset per run, so a run missing this case leaves a gap instead of
// shifting the colors of the runs that do have it.
function profileDatasets(points, endpoint, counts) {
  return points.map((point, index) => {
    const color = recencyColor(index, points.length);
    const value = point.summary.cases.find((item) => item.id === endpoint);
    return {
      ...chartDataset(
        pointLabel(point, counts),
        percentileSeries.map(({metric}) => value?.latencyMs?.[metric] ?? null),
        color,
        value ? statusColors(percentileSeries.map(() => value), color) : color
      ),
      borderWidth: 2
    };
  });
}

function createProfileChart(canvas, endpoint, points, counts) {
  charts.push(new Chart(canvas, {
    type: "line",
    data: {
      labels: percentileSeries.map(({label}) => label),
      datasets: profileDatasets(points, endpoint, counts)
    },
    options: panelOptions(endpoint, {autoSkip: false})
  }));
}

// The time-series twin of the percentile panel: same grid, x is the run date.
// Dates only on the axis - the full "date, revision" label is 20-plus
// characters and eats half a panel - with the whole label kept in the tooltip.
function createTimelineChart(canvas, endpoint, points, counts) {
  charts.push(new Chart(canvas, {
    type: "line",
    data: {
      labels: points.map((point) => point.entry.startedAt.slice(5, 10)),
      // Markers stop being readable once a quarter of nightlies is in the
      // window - they merge into a bead chain and hide the line - so past a
      // point the line alone carries the shape and hover carries the values.
      datasets: latencyDatasets(points.map(({value}) => value))
        .map((dataset) => ({
          ...dataset,
          borderWidth: 2,
          pointRadius: points.length > 30 ? 0 : 4,
          pointHoverRadius: 5,
          pointHitRadius: 8
        }))
    },
    options: panelOptions(
      endpoint,
      {autoSkip: true, maxRotation: 0, maxTicksLimit: 6},
      points.map((point) => pointLabel(point, counts))
    )
  }));
}

function median(values) {
  const sorted = [...values].sort((first, second) => first - second);
  const middle = sorted.length / 2;
  return sorted.length % 2 === 1
    ? sorted[Math.floor(middle)]
    : (sorted[middle - 1] + sorted[middle]) / 2;
}

function caseIn(run, endpoint) {
  return run.summary.cases.find((item) => item.id === endpoint);
}

// Scored against the spread of the case's own history rather than a flat
// percentage: a method that swings tenfold between runs needs a far bigger move
// to mean anything than one that repeats to within a percent.
function scoreMetric(metric, current, samples) {
  const value = metric.read(current);
  const history = samples
    .map(metric.read)
    .filter((sample) => typeof sample === "number" && sample > 0);
  if (typeof value !== "number" || value <= 0 ||
    history.length < minBaselineRuns) {
    return null;
  }
  const baseline = median(history);
  const scatter = Math.max(
    median(history.map((sample) => Math.abs(Math.log(sample / baseline)))),
    scatterFloor
  );
  const change = value / baseline - 1;
  const score = Math.log(value / baseline) /
    scatter * (metric.worseWhen === "lower" ? -1 : 1);
  return {metric: metric.label, baseline, value, change, score};
}

function moverFor(endpoint, latest, history) {
  const current = caseIn(latest, endpoint);
  if (!current || current.status !== "passed") {
    return null;
  }
  // Failed runs never set a baseline; they measure a different code path.
  const samples = history
    .map((run) => caseIn(run, endpoint))
    .filter((item) => item?.status === "passed");
  const scored = highlightMetrics
    .map((metric) => scoreMetric(metric, current, samples))
    .filter((entry) => entry &&
      Math.abs(entry.score) >= minScore &&
      Math.abs(entry.change) >= minChange);
  if (scored.length === 0) {
    return null;
  }
  return {
    id: endpoint,
    ...scored.reduce((worst, entry) =>
      Math.abs(entry.score) > Math.abs(worst.score) ? entry : worst
    )
  };
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
      datasets: latencyDatasets(values, false)
    },
    options: chartOptions("Endpoint latency (ms)", true, true)
  }));
}

function highlightHeading(text) {
  const heading = document.createElement("h3");
  heading.textContent = text;
  return heading;
}

function highlightNote(text) {
  const note = document.createElement("p");
  note.className = "highlight-note";
  note.textContent = text;
  return note;
}

function noteCard(text) {
  const panel = document.createElement("section");
  panel.append(highlightNote(text));
  return panel;
}

function endpointLink(endpoint) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "endpoint-link";
  button.textContent = endpoint;
  button.addEventListener("click", () => {
    endpointSelect.value = endpoint;
    render();
  });
  return button;
}

function moverRow(mover) {
  const row = document.createElement("tr");
  const endpoint = document.createElement("th");
  const change = document.createElement("td");
  const cells = [
    mover.metric,
    `${formatNumber(mover.baseline)} → ${formatNumber(mover.value)}`
  ].map((text) => {
    const cell = document.createElement("td");
    cell.textContent = text;
    return cell;
  });
  endpoint.scope = "row";
  endpoint.append(endpointLink(mover.id));
  change.className = mover.score > 0 ? "failed" : "passed";
  change.textContent = `${mover.change > 0 ? "+" : ""}` +
    `${formatNumber(mover.change * 100, 1)}%`;
  const deviation = document.createElement("td");
  deviation.textContent = `${formatNumber(Math.abs(mover.score), 1)}×`;
  row.append(endpoint, ...cells, change, deviation);
  return row;
}

function moverTable(movers) {
  const table = document.createElement("table");
  const head = document.createElement("thead");
  const body = document.createElement("tbody");
  const headings = document.createElement("tr");
  for (const label of ["Endpoint", "Metric", "Baseline → latest", "Change",
    "vs own spread"]) {
    const heading = document.createElement("th");
    heading.scope = "col";
    heading.textContent = label;
    headings.append(heading);
  }
  head.append(headings);
  body.append(...movers.map(moverRow));
  table.className = "movers";
  table.append(head, body);
  return table;
}

function renderHighlights() {
  highlightsElement.replaceChildren(highlightHeading("Highlights"));
  const latest = runs.at(-1);
  const history = runs.slice(0, -1);
  if (!latest) {
    highlightsElement.append(highlightNote("No runs in this window."));
    return;
  }
  if (history.length < minBaselineRuns) {
    highlightsElement.append(highlightNote(
      `Movers need ${minBaselineRuns} earlier runs to score against; ` +
      `${history.length} available in this window.`
    ));
    return;
  }

  // A mover exists only in the latest run, and moverFor already drops a case
  // that is failing now or has too few passing runs to set a baseline.
  const movers = latest.summary.cases
    .map((item) => moverFor(item.id, latest, history))
    .filter(Boolean)
    .sort((first, second) => Math.abs(second.score) - Math.abs(first.score));
  const worse = movers.filter((mover) => mover.score > 0);
  const better = movers.filter((mover) => mover.score < 0);
  if (movers.length === 0) {
    highlightsElement.append(highlightNote(
      "No endpoint moved beyond its usual run-to-run spread."
    ));
    return;
  }
  if (worse.length > 0) {
    highlightsElement.append(
      highlightHeading("Degradations"),
      moverTable(worse)
    );
  }
  if (better.length > 0) {
    highlightsElement.append(
      highlightHeading("Improvements"),
      moverTable(better)
    );
  }
}

function renderProfile() {
  const endpoint = endpointSelect.value === allEndpoints
    ? null
    : endpointSelect.value;
  const points = runs.slice(-Number(runCountSelect.value));
  // Panels come from the runs actually drawn, not from the whole window, so a
  // case the catalog has since dropped does not leave an empty panel behind.
  const drawn = new Set(points.flatMap(({summary}) =>
    summary.cases.map((item) => item.id)
  ));
  const selected = (endpoint ? [endpoint] : endpoints)
    .filter((item) => drawn.has(item));
  const counts = dateCounts(points);
  // The cards must describe a run that is actually plotted. Resolving the
  // endpoint against the whole window instead would date the whole block to
  // the last run that still carried the case - a "Latest p95" from weeks ago.
  const endpointLatest = endpoint
    ? points
      .map((point) => ({...point, value: caseIn(point, endpoint)}))
      .filter(({value}) => value)
      .at(-1)
    : null;
  populateLatest(
    endpointLatest ?? runs.at(-1),
    endpointLatest ? endpoint : null
  );
  statusElement.textContent = describeSelection(
    `${points.length} of ${runs.length} run(s) · ${selected.length} endpoint(s)`
  );
  showPanels("grid");
  if (points.length === 0) {
    overviewCharts.append(noteCard("No runs in this window."));
    return;
  }
  // Guarding on the panels rather than the runs, so a selection with nothing
  // to draw says why instead of leaving a legend above an empty grid.
  if (selected.length === 0) {
    overviewCharts.append(noteCard(endpoint
      ? `${endpoint} has no result in the latest ${points.length} run(s). ` +
        `Raise "Compare runs" or widen the history window.`
      : "These runs carry no endpoint results."));
    return;
  }

  const grid = chartGrid();
  overviewCharts.append(
    legendList(points.map((point, index) => ({
      label: pointLabel(point, counts),
      color: recencyColor(index, points.length)
    }))),
    grid
  );
  for (const item of selected) {
    createProfileChart(gridPanel(grid), item, points, counts);
  }
}

// All endpoints over time: one panel per endpoint so a trend reads left to
// right. Panel count is fixed at the endpoint count, where the per-run
// snapshot view instead grows by one chart every night.
function renderTimelineGrid() {
  const counts = dateCounts(runs);
  populateLatest(runs.at(-1));
  statusElement.textContent = describeSelection(
    `${runs.length} run(s) · ${endpoints.length} endpoint(s)`
  );
  showPanels("grid");
  if (runs.length === 0) {
    return;
  }

  const grid = chartGrid();
  overviewCharts.append(legendList(latencySeries), grid);
  for (const endpoint of endpoints) {
    const points = pointsFor(endpoint);
    if (points.length > 0) {
      createTimelineChart(gridPanel(grid), endpoint, points, counts);
    }
  }
}

function renderOverview() {
  populateLatest(runs.at(-1));
  statusElement.textContent = describeSelection(
    `${runs.length} run(s) · ${endpoints.length} endpoint(s)`
  );
  showPanels("grid");

  const counts = dateCounts(runs);
  for (const run of [...runs].reverse()) {
    createDailyChart(dailySection(run, counts), run);
  }
}

function renderEndpoint(endpoint) {
  const points = pointsFor(endpoint);
  populateLatest(points.at(-1), endpoint);
  statusElement.textContent = describeSelection(
    `${points.length} run(s) · ${endpoint}`
  );
  showPanels("single");
  createLatencyChart(document.getElementById("latency"), endpoint, points);
  createThroughputChart(document.getElementById("throughput"), endpoint, points);
}

function skippedLabel() {
  return skippedRuns === 0 ? "" : ` · ${skippedRuns} invalid run(s) skipped`;
}

// "grid" for the panel grids and the per-run stack, "single" for the
// full-width endpoint charts.
function showPanels(target) {
  overviewCharts.hidden = target !== "grid";
  latencyPanel.hidden = target !== "single";
  throughputPanel.hidden = target !== "single";
}

function describeSelection(lead) {
  return `${lead} · ${windowSelect.value}-month window${skippedLabel()}`;
}

function render() {
  destroyCharts();
  renderHighlights();
  runCountControl.hidden = viewSelect.value !== "percentile";
  if (viewSelect.value === "percentile") {
    renderProfile();
  } else if (viewSelect.value === "snapshot") {
    renderOverview();
  } else if (endpointSelect.value === allEndpoints) {
    renderTimelineGrid();
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
viewSelect.addEventListener("change", render);
runCountSelect.addEventListener("change", render);
windowSelect.addEventListener("change", reload);
reload();
