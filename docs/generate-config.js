const fs = require("fs");
const https = require("https");

function preprocessCodebase(codebase) {
  // Split the codebase into lines
  const lines = codebase.split("\n");
  let mergedLines = [];
  let i = 0;

  // Iterate through each line
  while (i < lines.length) {
    let currentLine = lines[i].trimRight();

    // If the line ends with '+', merge it with the next line
    while (currentLine.endsWith("+")) {
      i++;
      if (i < lines.length) {
        // Merge the current line with the next line, removing excess double quotes
        currentLine = (
          currentLine.slice(0, -1).trim() + lines[i].trim()
        ).replace(/""/g, "");
      } else {
        break;
      }
    }

    // Add the processed line to the merged lines array
    mergedLines.push(currentLine);
    i++;
  }

  // Join the merged lines back into a single string
  return mergedLines.join("\n");
}

const extractConfigs = (codebase) => {
  // Pattern to match variable declarations
  const variablesPattern = /(\w+)\s*(=|:=)\s*(.*?)(?:\s*\/\/.*)?$/gm;
  const variables = {};

  // Extract variables
  let match;
  while ((match = variablesPattern.exec(codebase)) !== null) {
    const [_, varName, __, value] = match;
    variables[varName] = parseValue(value); // Ensure you define `parseValue` function to handle the values correctly
  }

  // Walk lines to (a) track the current category from `// --- <Title> ---`
  // dividers and (b) collect multi-line `junoCmd.Flags().XXX(` calls into a
  // single buffer so the regex below can match registrations that span lines.
  const lines = codebase.split("\n");
  const dividerPattern = /^\s*\/\/\s*---\s*(.+?)\s*---\s*$/;
  const flagOpenPattern = /^\s*junoCmd\.Flags\(\)\./;
  const flagPattern = /junoCmd\.Flags\(\)\.[A-Za-z\d]+\((.*)\)/;
  const argsPattern = /([^\s,]+)/g;
  const configs = [];
  let currentSection = "Other";

  for (let i = 0; i < lines.length; i++) {
    const dividerMatch = lines[i].match(dividerPattern);
    if (dividerMatch) {
      currentSection = dividerMatch[1];
      continue;
    }
    if (!flagOpenPattern.test(lines[i])) continue;

    // Collect lines until parens balance, so multi-line registrations like
    //   junoCmd.Flags().Duration(\n  fooF,\n  defaultFoo,\n  fooUsage,\n)
    // become a single buffer the existing flagPattern can match.
    let buffer = lines[i];
    let opens = (buffer.match(/\(/g) || []).length;
    let closes = (buffer.match(/\)/g) || []).length;
    while (opens > closes && i + 1 < lines.length) {
      i++;
      buffer += " " + lines[i].trim();
      opens += (lines[i].match(/\(/g) || []).length;
      closes += (lines[i].match(/\)/g) || []).length;
    }

    const flagMatch = buffer.match(flagPattern);
    if (!flagMatch) continue;
    const flags = flagMatch[1];
    const args = [...flags.matchAll(argsPattern)].map((m) => m[0]);
    if (args.length < 3) continue;

    let configName, defaultValue, description;
    if (args[args.length - 3][0] === "&") {
      configName = variables[args[args.length - 2]];
      defaultValue = variables[args[args.length - 3].slice(1)];
    } else {
      configName = variables[args[args.length - 3]];
      defaultValue = variables[args[args.length - 2]];
    }

    if (defaultValue === undefined) {
      defaultValue = "";
    }

    description = variables[args[args.length - 1]] || "";
    description = description.replace(/\.$/, ""); // Remove any trailing dot

    // Additional descriptions based on specific configurations
    if (configName === "max-vms") {
      defaultValue = "3 * CPU Cores";
    }
    if (configName === "max-vm-queue") {
      defaultValue = "2 * max-vms";
    }
    if (configName === "gw-timeouts") {
      defaultValue = "5s";
    }
    configs.push({
      configName,
      defaultValue,
      description,
      section: currentSection,
    });
  }

  return configs;
};

function parseValue(value) {
  // Trim the value to remove whitespace
  value = value.trim();

  // Check if the value is a digit (considering underscores in numeric literals)
  if (/^\d+$/.test(value.replace(/_/g, ""))) {
    return parseInt(value.replace(/_/g, ""), 10);
  }

  // Check for boolean values
  if (value.toLowerCase() === "true" || value.toLowerCase() === "false") {
    return value.toLowerCase() === "true";
  }

  // Check for empty integer array
  if (value === "[]int{}") {
    return "[]";
  }

  // Handle time duration represented in seconds
  if (value.includes("* time.Second")) {
    return value.split(" ")[0];
  }

  // Handle large unsigned integer value
  if (value === "math.MaxUint") {
    return "18446744073709551615";
  }

  // Custom path case
  if (value.includes("filepath.Join(defaultDBPath")) {
    return "juno";
  }

  // Logging level
  if (value === "log.INFO") {
    return "info";
  }

  // Network type
  if (value === "networks.Mainnet") {
    return "mainnet";
  }

  // Remove quotes from a string
  if (value.startsWith('"') && value.endsWith('"')) {
    return value.slice(1, -1);
  }

  // Return the value directly if it's not empty, otherwise return null
  return value || null;
}

function formatDefault(defaultValue) {
  if (defaultValue === "") return "";
  if (typeof defaultValue === "boolean") {
    return `\`${defaultValue}\``.toLowerCase();
  }
  return `\`${defaultValue}\``;
}

function generateConfigTable(configs) {
  // Group flags by section, preserving the order in which sections first
  // appeared in the source — that order matches the `// --- <Title> ---`
  // dividers in cmd/juno/juno.go, which match the `--help` render order.
  const sectionOrder = [];
  const bySection = new Map();
  for (const config of configs) {
    if (!bySection.has(config.section)) {
      sectionOrder.push(config.section);
      bySection.set(config.section, []);
    }
    bySection.get(config.section).push(config);
  }

  const sections = sectionOrder.map((section) => {
    const rows = bySection
      .get(section)
      .slice()
      .sort((a, b) => a.configName.localeCompare(b.configName))
      .map(
        (c) =>
          `| \`${c.configName}\` | ${formatDefault(c.defaultValue)} | ${c.description} |`,
      )
      .join("\n");
    return `### ${section}\n\n| Config Option | Default Value | Description |\n| - | - | - |\n${rows}\n`;
  });

  const fileWarning =
    "<!-- This file is generated automatically. Any manual modifications will be overwritten. -->\n\n";
  fs.writeFileSync(
    "docs/_config-options.md",
    fileWarning + sections.join("\n"),
  );
}

function fetchUrl(url) {
  return new Promise((resolve, reject) => {
    https
      .get(url, (res) => {
        if (res.statusCode !== 200) {
          reject(new Error(`HTTP status ${res.statusCode}`));
          res.resume(); // Consume response data to free up memory
          return;
        }

        res.setEncoding("utf8");
        let rawData = "";
        res.on("data", (chunk) => {
          rawData += chunk;
        });
        res.on("end", () => {
          resolve(rawData);
        });
      })
      .on("error", (e) => {
        reject(e);
      });
  });
}

async function main() {
  try {
    const url =
      "https://raw.githubusercontent.com/NethermindEth/juno/main/cmd/juno/juno.go";
    const codebase = await fetchUrl(url);
    console.log("Fetched Juno's source code");

    const preprocessedCode = preprocessCodebase(codebase);
    const configs = extractConfigs(preprocessedCode);
    console.log("Extracted Juno's configuration");

    generateConfigTable(configs);
    console.log("Generated the configuration options table");
  } catch (error) {
    console.error(
      "An error occurred while generating the config: ",
      error.message,
    );
  }
}

main();
