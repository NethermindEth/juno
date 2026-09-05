#!/usr/bin/env node
// Verifies the docs and the binary agree about flags, in both directions. Next docs
// are checked against the working tree; published docs against their line's newest stable tag.

const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");
const {
  preprocessCodebase,
  extractConfigs,
  generateConfigTable,
} = require("./generate-config.js");

const repoRoot = path.resolve(__dirname, "..");
// Registered by cobra itself, so real flags without a Flags() call in juno.go.
const COBRA_BUILTINS = new Set(["help", "version"]);

const findings = [];
const fail = (file, msg) =>
  findings.push(`${path.relative(repoRoot, file)}: ${msg}`);

function flagNames(goSource) {
  const names = extractConfigs(preprocessCodebase(goSource))
    .map((c) => c.configName)
    .filter((n) => typeof n === "string");
  return new Set([...names, ...COBRA_BUILTINS]);
}

// Flags a fenced line passes to Juno. Docker's own flags sit before the image,
// and helm charts are also named nethermind/juno, so the image rule is docker-only.
function flagsFromCommand(line) {
  // Env-var prefixes (`JUNO_HTTP=true juno ...`) would otherwise hide the command.
  let args = line.trim().replace(/^(?:[A-Z_][A-Z0-9_]*=\S+\s+)+/, "");
  // Anchored so nethermind/juno-plugin and friends do not match.
  const image =
    /\bdocker\s+(run|create)\b/.test(args) &&
    args.match(/nethermind\/juno(?::\S+|@\S+)?(?=\s|$)/);
  if (image) {
    args = args.slice(image.index + image[0].length);
  } else if (/^(\S*\/)?juno\s/.test(args)) {
    args = args.replace(/^(\S*\/)?juno\s+/, "");
  } else {
    return [];
  }
  return [...args.matchAll(/(?:^|\s)--([a-z][a-z0-9-]*)/g)].map((m) => m[1]);
}

// Every --flag a page mentions: backticked in prose, or passed to Juno in a fence.
function flagMentions(markdown) {
  const mentions = new Map(); // name -> first line, for the report
  const lines = markdown.split("\n");
  let fenceMarker = null; // a fence only closes on its own marker
  let joined = "";
  for (let i = 0; i < lines.length; i++) {
    const l = lines[i];
    const fence = l.trim().match(/^(```|~~~)/);
    if (fence && (!fenceMarker || fence[1] === fenceMarker)) {
      fenceMarker = fenceMarker ? null : fence[1];
      joined = "";
      continue;
    }
    const inFence = fenceMarker !== null;
    if (inFence) {
      joined += l.endsWith("\\") ? l.slice(0, -1) + " " : l;
      if (l.endsWith("\\")) continue;
      for (const f of flagsFromCommand(joined)) {
        if (!mentions.has(f)) mentions.set(f, i + 1);
      }
      joined = "";
    } else {
      // A removed or renamed flag must stay documentable, so those lines are exempt.
      if (/\b(removed|renamed|deprecated)\b/i.test(l)) continue;
      for (const m of l.matchAll(/`--([a-z][a-z0-9-]*)[^`]*`/g)) {
        if (!mentions.has(m[1])) mentions.set(m[1], i + 1);
      }
    }
  }
  return mentions;
}

// First cell of each row in a generated config table.
function tableNames(markdown) {
  const names = new Map();
  markdown.split("\n").forEach((l, i) => {
    const m = l.match(/^\| `([a-z0-9-]+)` \|/);
    if (m && !names.has(m[1])) names.set(m[1], i + 1);
  });
  return names;
}

function checkTree(treeDir, source) {
  const names = flagNames(source.text);

  // Documented flags must exist in the binary: the --log-port class.
  for (const file of fs.readdirSync(treeDir).filter((f) => f.endsWith(".md"))) {
    const p = path.join(treeDir, file);
    const md = fs.readFileSync(p, "utf8");
    const documented = file.startsWith("_") ? tableNames(md) : flagMentions(md);
    for (const [flag, line] of documented) {
      if (!names.has(flag)) {
        fail(p, `line ${line}: \`--${flag}\` is not a flag in ${source.label}`);
      }
    }
  }

  // Registered flags must be in the table: the undocumented-flag class.
  const tableFile = path.join(treeDir, "_config-options.md");
  if (!fs.existsSync(tableFile)) {
    fail(tableFile, "config table missing from this tree");
    return;
  }
  const inTable = tableNames(fs.readFileSync(tableFile, "utf8"));
  for (const name of names) {
    if (COBRA_BUILTINS.has(name)) continue;
    if (!inTable.has(name)) {
      fail(tableFile, `missing \`${name}\`, which ${source.label} registers`);
    }
  }
}

function main() {
  // next: the working tree is the truth.
  const localSource = fs.readFileSync(
    path.join(repoRoot, "cmd", "juno", "juno.go"),
    "utf8",
  );
  checkTree(path.join(__dirname, "docs"), {
    label: "cmd/juno/juno.go (working tree)",
    text: localSource,
  });

  // next only: the committed table must byte-match the generator's output.
  const expected = generateConfigTable(
    extractConfigs(preprocessCodebase(localSource)),
  );
  const tablePath = path.join(__dirname, "docs", "_config-options.md");
  if (fs.readFileSync(tablePath, "utf8") !== expected) {
    fail(
      tablePath,
      "stale against cmd/juno/juno.go (working tree); run `cd docs && node generate-config.js` and commit the result",
    );
  }

  // published: checked against the newest stable tag of its line.
  const published = JSON.parse(
    fs.readFileSync(path.join(__dirname, "versions.json"), "utf8"),
  )[0];
  const line = published.split(".").slice(0, 2).join(".");
  const tags = execFileSync("git", ["tag", "-l", `v${line}.*`], {
    cwd: repoRoot,
  })
    .toString()
    .split("\n")
    .filter((t) => /^v\d+\.\d+\.\d+$/.test(t)) // stable releases only
    .sort((a, b) => Number(a.split(".")[2]) - Number(b.split(".")[2]));
  const tag = tags[tags.length - 1];
  if (!tag) {
    // Shallow clone without tags: warn instead of failing; CI fetches full history.
    console.warn(`no v${line}.* tag found; skipping the published-version check`);
  } else {
    const tagSource = execFileSync(
      "git",
      ["show", `${tag}:cmd/juno/juno.go`],
      { cwd: repoRoot },
    ).toString();
    checkTree(path.join(__dirname, "versioned_docs", `version-${published}`), {
      label: `${tag} (newest stable of the ${line} line)`,
      text: tagSource,
    });
  }

  if (findings.length) {
    console.error(`${findings.length} finding(s):\n`);
    for (const f of findings) console.error(`  FAIL ${f}`);
    process.exit(1);
  }
  console.log(
    `docs and binary agree on flags (next vs cmd/juno/juno.go, ${published} vs ${tag || "skipped"})`,
  );
}

try {
  main();
} catch (err) {
  // Environment problems (no git, broken tree) are not docs findings.
  console.error(`check-flags: ${err.message}`);
  process.exit(2);
}
