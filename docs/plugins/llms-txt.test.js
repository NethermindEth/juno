// One case per defect cleanBody has actually had, so none of them come back.
const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { cleanBody } = require("./llms-txt.js");

test("strips MDX imports in prose", () => {
  const out = cleanBody(
    'import Tabs from "@theme/Tabs";\nimport C from "@site/x";\n\nProse.',
  );
  assert.ok(!out.includes("import"));
  assert.match(out, /Prose\./);
});

test("keeps an import inside a code fence", () => {
  const out = cleanBody('```go\nimport (\n\t"fmt"\n)\n```');
  assert.match(out, /import \(/);
  assert.match(out, /"fmt"/);
});

test("a tilde line inside a backtick fence does not flip fence state", () => {
  const out = cleanBody(
    '```bash\n~~~\n```\nimport Tabs from "@theme/Tabs";\n\nProse.',
  );
  assert.ok(!out.includes("import Tabs"));
});

test("keeps an import inside a tilde fence", () => {
  const out = cleanBody('~~~js\nimport x from "y";\n~~~');
  assert.match(out, /import x from "y";/);
});

test("unwraps a top-level mdx-code-block fence", () => {
  const out = cleanBody("```mdx-code-block\nkept\n```");
  assert.match(out, /kept/);
  assert.ok(!out.includes("mdx-code-block"));
});

test("an mdx-code-block opener inside a real fence does not corrupt state", () => {
  const out = cleanBody('~~~\n```mdx-code-block\n~~~\nimport a from "b";\n\nProse.');
  assert.ok(!out.includes("import a"));
  assert.match(out, /```mdx-code-block/);
});

test("turns a GuideCard into a markdown link", () => {
  const out = cleanBody(
    '<GuideCard href="running-juno" title="Running Juno" description="Set up a node" />',
  );
  assert.match(out, /- \[Running Juno\]\(running-juno\): Set up a node/);
});

test("accepts label= as well as value= on a TabItem", () => {
  const out = cleanBody('<Tabs>\n<TabItem label="Binary">\nx\n</TabItem>\n</Tabs>');
  assert.match(out, /^### Binary$/m);
  assert.ok(!/<\/?(Tabs|TabItem)\b/.test(out));
});

test("leaves the tag alone when the imported partial is missing", () => {
  const out = cleanBody('import Gone from "./_gone.md";\n\n<Gone />', "/nonexistent");
  assert.match(out, /<Gone \/>/);
  assert.ok(!out.includes("import Gone"));
});

test("leaves a malformed GuideCard visible instead of dropping it", () => {
  const out = cleanBody('<GuideCard href="running-juno" ttile="typo" />');
  assert.match(out, /<GuideCard/);
});

test("unwraps TabItems to headings and drops the Tabs wrapper", () => {
  const out = cleanBody(
    '<Tabs>\n<TabItem value="docker">\ncontent\n</TabItem>\n</Tabs>',
  );
  assert.match(out, /^### docker$/m);
  assert.ok(!/<\/?(Tabs|TabItem)\b/.test(out));
  assert.match(out, /content/);
});

test("splices a local partial in at its component tag", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "llms-"));
  fs.writeFileSync(path.join(dir, "_table.md"), "| `http-port` | `6060` |\n");
  const out = cleanBody(
    'import Table from "./_table.md";\n\nBelow:\n\n<Table />',
    dir,
  );
  assert.match(out, /\| `http-port` \| `6060` \|/);
  assert.ok(!out.includes("<Table />"));
  fs.rmSync(dir, { recursive: true, force: true });
});

test("collapses runs of blank lines", () => {
  assert.ok(!cleanBody("a\n\n\n\n\nb").includes("\n\n\n"));
});
