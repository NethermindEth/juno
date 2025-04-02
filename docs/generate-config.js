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

  // Pattern to find flags used in Juno's command-line configurations
  const flagPattern = /junoCmd\.Flags\(\)\.[A-Za-z\d]+\((.*)\)/g;
  const argsPattern = /([^\s,]+)/g;
  const configs = [];

  while ((match = flagPattern.exec(codebase)) !== null) {
    const flags = match[1];
    const args = [...flags.matchAll(argsPattern)].map((m) => m[0]);

    if (args.length >= 3) {
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
      });
    }
  }

  // Sort configurations by name
  return configs.sort((a, b) => a.configName.localeCompare(b.configName));
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
  if (value === "utils.INFO") {
    return "info";
  }

  // Network type
  if (value === "utils.Mainnet") {
    return "mainnet";
  }

  // Remove quotes from a string
  if (value.startsWith('"') && value.endsWith('"')) {
    return value.slice(1, -1);
  }

  // Return the value directly if it's not empty, otherwise return null
  return value || null;
}

function generateConfigTable(configs) {
  let configTable =
    "| Config Option | Default Value | Description |\n| - | - | - |\n";
  configs.forEach((config) => {
    let defaultValue = config.defaultValue;
    if (defaultValue !== "") {
      if (typeof defaultValue === "boolean") {
        defaultValue = `\`${defaultValue}\``.toLowerCase();
      } else {
        defaultValue = `\`${defaultValue}\``;
      }
    }
    configTable += `| \`${config.configName}\` | ${defaultValue} | ${config.description} |\n`;
  });

  // Write to file
  let fileWarning =
    "<!-- This file is generated automatically. Any manual modifications will be overwritten. -->\n\n";
  fs.writeFileSync("docs/_config-options.md", fileWarning + configTable);
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
