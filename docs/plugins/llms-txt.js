// Emits /llms.txt, /llms-full.txt and a raw .md route beside every page, on every build,
// generated from the version the site root serves (versions.json[0]), never from next.

const fs = require("fs");
const path = require("path");

// Strip MDX plumbing (imports, mdx-code-block fences) so agents get plain markdown.
// dir is the page's directory, used to inline any local partial it renders.
function cleanBody(body, dir) {
  // Inline local partials: `import X from "./_p.md"` + `<X />` becomes the
  // partial's content, so a page's real table isn't left as a dangling tag.
  if (dir) {
    const partials = {};
    for (const m of body.matchAll(/^import\s+(\w+)\s+from\s+["'](\.\/[\w./-]+\.md)["']/gm)) {
      const f = path.join(dir, m[2]);
      if (fs.existsSync(f)) partials[m[1]] = readPage(f).body;
    }
    for (const [name, pbody] of Object.entries(partials)) {
      body = body.replace(new RegExp(`<${name}\\s*/>`, "g"), "\n" + pbody.trim() + "\n");
    }
  }
  // Tabs are presentation only; unwrap TabItems to "### <label>" and drop the wrappers.
  body = body
    .replace(/<TabItem\b[^>]*\b(?:label|value)="([^"]+)"[^>]*>/g, "\n### $1\n")
    .replace(/<\/?Tabs\b[^>]*>/g, "")
    .replace(/<\/TabItem>/g, "");
  // GuideCards become plain markdown links so their routing survives.
  body = body.replace(/<GuideCard\b([\s\S]*?)\/>/g, (m, props) => {
    const g = (k) => (props.match(new RegExp(`${k}="([^"]+)"`)) || [])[1];
    if (!g("href") || !g("title")) return m; // keep a broken card visible rather than dropping it
    return `- [${g("title")}](${g("href")})${g("description") ? ": " + g("description") : ""}`;
  });
  const lines = [];
  let inMdx = false;
  let inFence = false; // never strip anything inside a real code fence
  for (const l of body.split("\n")) {
    const t = l.trim();
    if (t === "```mdx-code-block") { inMdx = true; continue; }
    if (inMdx && t === "```") { inMdx = false; continue; }
    if (!inMdx && /^(```|~~~)/.test(t)) inFence = !inFence;
    if (!inFence && /^import\s.+\sfrom\s["']/.test(l)) continue;
    lines.push(l);
  }
  return lines.join("\n").replace(/\n{3,}/g, "\n\n");
}

// Read one page's frontmatter and body.
function readPage(file) {
  const raw = fs.readFileSync(file, "utf8");
  const fm = raw.match(/^---\n([\s\S]*?)\n---\n/);
  const field = (name) => {
    const v = ((fm ? fm[1] : "").match(new RegExp(`^${name}:\\s*(.+)$`, "m")) || [])[1];
    return v && v.replace(/^"(.*)"$/, "$1"); // values may be quoted
  };
  return {
    id: path.basename(file, ".md"),
    title: field("title") || path.basename(file, ".md"),
    description: field("description") || "",
    slug: field("slug"),
    raw,
    body: fm ? raw.slice(fm[0].length) : raw,
  };
}

// Sidebar JSON: strings are doc ids, categories carry labels, html items are skipped.
function sidebarSections(sidebarFile) {
  const sidebars = Object.values(JSON.parse(fs.readFileSync(sidebarFile, "utf8")));
  if (sidebars.length !== 1) {
    throw new Error(`[llms-txt] expected one sidebar, found ${sidebars.length}; update the plugin`);
  }
  const sidebar = sidebars[0];
  const sections = [];
  let current = { label: null, ids: [] };
  for (const item of sidebar) {
    if (typeof item === "string") {
      current.ids.push(item);
    } else if (item.type === "category") {
      if (current.ids.length) sections.push(current);
      const ids = item.items.filter((i) => typeof i === "string");
      sections.push({ label: item.label, ids });
      current = { label: null, ids: [] };
    }
  }
  if (current.ids.length) sections.push(current);
  return sections;
}

module.exports = function llmsTxtPlugin() {
  return {
    name: "llms-txt",
    async postBuild({ siteConfig, siteDir, outDir }) {
      const versions = JSON.parse(
        fs.readFileSync(path.join(siteDir, "versions.json"), "utf8"),
      );
      const version = versions[0];
      const docsDir = path.join(siteDir, "versioned_docs", `version-${version}`);
      const sidebarFile = path.join(
        siteDir, "versioned_sidebars", `version-${version}-sidebars.json`,
      );
      const site = siteConfig.url;

      // Disk decides what exists; the sidebar only orders and labels it.
      const onDisk = fs.readdirSync(docsDir)
        .filter((f) => f.endsWith(".md") && !f.startsWith("_"))
        .map((f) => f.slice(0, -3));
      const sections = sidebarSections(sidebarFile);
      const listed = new Set(sections.flatMap((s) => s.ids));
      const unlisted = onDisk.filter((id) => !listed.has(id));
      if (unlisted.length) sections.push({ label: "Other pages", ids: unlisted.sort() });

      const index = [];
      const full = [];
      for (const section of sections) {
        if (section.label) index.push(`\n- ${section.label}\n`);
        for (const id of section.ids) {
          const file = path.join(docsDir, `${id}.md`);
          if (!fs.existsSync(file)) {
            console.warn(`[llms-txt] sidebar lists "${id}" but ${file} does not exist; skipping`);
            continue;
          }
          const page = readPage(file);
          const route = page.slug === "/" ? "" : (page.slug || `/${id}`);
          const url = `${site}${route || "/"}`;
          const note = page.description ? `: ${page.description}` : "";
          index.push(`  - [${page.title}](${url})${note}`);
          const body = cleanBody(page.body, docsDir).trim();
          full.push(`# ${page.title}\n\nSource: ${url}\n\n${body}\n`);
          // Raw markdown sits beside its HTML route; the front page becomes /index.md.
          const clean = body.startsWith("# ")
            ? `${body}\n`
            : `# ${page.title}\n\n${page.description ? page.description + "\n\n" : ""}${body}\n`;
          const target = path.join(outDir, `${route === "" ? "index" : route.slice(1)}.md`);
          fs.mkdirSync(path.dirname(target), { recursive: true });
          fs.writeFileSync(target, clean);
          if (route === "") fs.writeFileSync(path.join(outDir, `${id}.md`), clean);
        }
      }

      const header =
        "Append `.md` to any docs page URL below for its raw Markdown " +
        `(e.g. \`${site}/configuring.md\`; the front page is \`${site}/index.md\`), ` +
        `or fetch all current pages at \`${site}/llms-full.txt\`.\n\n` +
        `# Juno Docs\n\n> ${siteConfig.title}: ${siteConfig.tagline}. ` +
        `Documentation for Juno ${version}, a Starknet full node written in Go.\n`;

      fs.writeFileSync(path.join(outDir, "llms.txt"), header + "\n" + index.join("\n") + "\n");
      fs.writeFileSync(path.join(outDir, "llms-full.txt"), full.join("\n---\n\n"));
      console.log(`[llms-txt] wrote llms.txt, llms-full.txt and raw .md routes for ${version}`);
    },
  };
};
