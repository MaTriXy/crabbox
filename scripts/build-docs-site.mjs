#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const docsDir = path.join(root, "docs");
const outDir = path.join(root, "dist", "docs-site");

const sections = [
  ["Start", ["README.md", "architecture.md", "orchestrator.md", "cli.md"]],
  ["Features", rels("features")],
  ["Commands", rels("commands")],
  ["Operate", ["infrastructure.md", "security.md", "mvp-plan.md"]],
];

fs.rmSync(outDir, { recursive: true, force: true });
fs.mkdirSync(outDir, { recursive: true });

const pages = allMarkdown(docsDir).map((file) => {
  const rel = path.relative(docsDir, file).replaceAll(path.sep, "/");
  const markdown = fs.readFileSync(file, "utf8");
  const title = firstHeading(markdown) || titleize(path.basename(rel, ".md"));
  return { file, rel, title, outRel: outPath(rel), markdown };
});

const pageMap = new Map(pages.map((page) => [page.rel, page]));
const nav = sections
  .map(([name, rels]) => ({
    name,
    pages: rels.map((rel) => pageMap.get(rel)).filter(Boolean),
  }))
  .filter((section) => section.pages.length);

for (const page of pages) {
  const html = markdownToHtml(page.markdown, page.rel);
  const pageOut = path.join(outDir, page.outRel);
  fs.mkdirSync(path.dirname(pageOut), { recursive: true });
  fs.writeFileSync(pageOut, layout(page, html), "utf8");
}

fs.writeFileSync(path.join(outDir, "crabbox.svg"), crabSvg(), "utf8");
fs.writeFileSync(path.join(outDir, ".nojekyll"), "", "utf8");
console.log(`built docs site: ${path.relative(root, outDir)}`);

function rels(dir) {
  const full = path.join(docsDir, dir);
  if (!fs.existsSync(full)) return [];
  return fs
    .readdirSync(full)
    .filter((name) => name.endsWith(".md"))
    .sort((a, b) => (a === "README.md" ? -1 : b === "README.md" ? 1 : a.localeCompare(b)))
    .map((name) => `${dir}/${name}`);
}

function allMarkdown(dir) {
  return fs
    .readdirSync(dir, { withFileTypes: true })
    .flatMap((entry) => {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) return allMarkdown(full);
      return entry.name.endsWith(".md") ? [full] : [];
    })
    .sort();
}

function outPath(rel) {
  if (rel === "README.md") return "index.html";
  if (rel.endsWith("/README.md")) return rel.replace(/README\.md$/, "index.html");
  return rel.replace(/\.md$/, ".html");
}

function firstHeading(markdown) {
  return markdown.match(/^#\s+(.+)$/m)?.[1]?.trim();
}

function titleize(input) {
  return input.replaceAll("-", " ").replace(/\b\w/g, (m) => m.toUpperCase());
}

function markdownToHtml(markdown, currentRel) {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  const html = [];
  let paragraph = [];
  let list = null;
  let fence = null;

  const flushParagraph = () => {
    if (!paragraph.length) return;
    html.push(`<p>${inline(paragraph.join(" "), currentRel)}</p>`);
    paragraph = [];
  };
  const closeList = () => {
    if (!list) return;
    html.push(`</${list}>`);
    list = null;
  };

  for (const line of lines) {
    const fenceMatch = line.match(/^```(\w+)?\s*$/);
    if (fenceMatch) {
      flushParagraph();
      closeList();
      if (fence) {
        html.push(`<pre><code class="language-${fence.lang}">${escapeHtml(fence.lines.join("\n"))}</code></pre>`);
        fence = null;
      } else {
        fence = { lang: fenceMatch[1] || "text", lines: [] };
      }
      continue;
    }
    if (fence) {
      fence.lines.push(line);
      continue;
    }
    if (!line.trim()) {
      flushParagraph();
      closeList();
      continue;
    }
    const heading = line.match(/^(#{1,4})\s+(.+)$/);
    if (heading) {
      flushParagraph();
      closeList();
      const level = heading[1].length;
      const text = heading[2].trim();
      html.push(`<h${level} id="${slug(text)}">${inline(text, currentRel)}</h${level}>`);
      continue;
    }
    const bullet = line.match(/^\s*-\s+(.+)$/);
    const numbered = line.match(/^\s*\d+\.\s+(.+)$/);
    if (bullet || numbered) {
      flushParagraph();
      const tag = bullet ? "ul" : "ol";
      if (list && list !== tag) closeList();
      if (!list) {
        list = tag;
        html.push(`<${tag}>`);
      }
      html.push(`<li>${inline((bullet || numbered)[1], currentRel)}</li>`);
      continue;
    }
    paragraph.push(line.trim());
  }
  flushParagraph();
  closeList();
  return html.join("\n");
}

function inline(text, currentRel) {
  const stash = [];
  let out = text.replace(/`([^`]+)`/g, (_, code) => {
    stash.push(`<code>${escapeHtml(code)}</code>`);
    return `\u0000${stash.length - 1}\u0000`;
  });
  out = escapeHtml(out)
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, label, href) => `<a href="${escapeAttr(rewriteHref(href, currentRel))}">${label}</a>`);
  return out.replace(/\u0000(\d+)\u0000/g, (_, i) => stash[Number(i)]);
}

function rewriteHref(href, currentRel) {
  if (/^(https?:|mailto:|#)/.test(href)) return href;
  const [raw, hash = ""] = href.split("#");
  if (!raw) return `#${hash}`;
  if (!raw.endsWith(".md")) return href;
  const from = path.posix.dirname(currentRel);
  const target = path.posix.normalize(path.posix.join(from, raw));
  let rewritten = outPath(target);
  const currentOut = outPath(currentRel);
  rewritten = path.posix.relative(path.posix.dirname(currentOut), rewritten) || "index.html";
  return `${rewritten}${hash ? `#${hash}` : ""}`;
}

function layout(page, content) {
  const depth = page.outRel.split("/").length - 1;
  const rootPrefix = depth ? "../".repeat(depth) : "";
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>${escapeHtml(page.title)} · Crabbox Docs</title>
  <link rel="icon" href="${rootPrefix}crabbox.svg">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Fraunces:wght@650;760&family=IBM+Plex+Sans:wght@400;500;600;700&family=IBM+Plex+Mono:wght@500;600&display=swap" rel="stylesheet">
  <style>${css()}</style>
</head>
<body>
  <div class="shell">
    <aside class="sidebar">
      <a class="brand" href="${rootPrefix}index.html" aria-label="Crabbox docs home">
        <img src="${rootPrefix}crabbox.svg" alt="">
        <span><strong>Crabbox</strong><small>Remote testbox docs</small></span>
      </a>
      <label class="search"><span>Search docs</span><input id="doc-search" type="search" placeholder="leases, cost, ssh"></label>
      <nav>${navHtml(page.rel, rootPrefix)}</nav>
    </aside>
    <main>
      <header class="hero">
        <div>
          <p class="eyebrow">Crustacean control plane</p>
          <h1>${escapeHtml(page.title)}</h1>
        </div>
        <a class="repo" href="https://github.com/openclaw/crabbox">GitHub</a>
      </header>
      <article class="doc">${content}</article>
    </main>
  </div>
  <script>${js()}</script>
</body>
</html>`;
}

function navHtml(currentRel, rootPrefix) {
  return nav
    .map((section) => `<section><h2>${section.name}</h2>${section.pages.map((page) => {
      const href = rootPrefix + page.outRel;
      const active = page.rel === currentRel ? " active" : "";
      return `<a class="nav-link${active}" href="${href}">${escapeHtml(page.title)}</a>`;
    }).join("")}</section>`)
    .join("");
}

function css() {
  return `
:root{--ink:#12211f;--muted:#5d6e69;--shell:#f7efe3;--paper:#fffbf4;--reef:#145f58;--tide:#1f7b93;--coral:#e35e46;--ochre:#c89231;--line:#dfd2c0;--shadow:0 24px 70px rgba(18,33,31,.14)}
*{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;background:var(--shell);color:var(--ink);font-family:"IBM Plex Sans",Avenir Next,sans-serif;line-height:1.6;overflow-x:hidden}
body:before{content:"";position:fixed;inset:0;pointer-events:none;background:linear-gradient(90deg,rgba(18,33,31,.045) 1px,transparent 1px),linear-gradient(rgba(18,33,31,.035) 1px,transparent 1px);background-size:44px 44px;mask-image:linear-gradient(90deg,#000,transparent 72%)}
a{color:var(--reef);text-decoration-thickness:.08em;text-underline-offset:.18em}.shell{display:grid;grid-template-columns:310px minmax(0,1fr);min-height:100vh}
.sidebar{position:sticky;top:0;height:100vh;overflow:auto;padding:26px 22px;background:rgba(255,251,244,.82);border-right:1px solid var(--line);backdrop-filter:blur(20px)}
.brand{display:flex;align-items:center;gap:12px;color:var(--ink);text-decoration:none;margin-bottom:24px}.brand img{width:48px;height:48px}.brand strong{display:block;font-family:Fraunces,serif;font-size:1.45rem;line-height:1}.brand small{display:block;color:var(--muted);font-size:.78rem;margin-top:4px}
.search{display:block;margin:0 0 24px}.search span{display:block;color:var(--muted);font-size:.76rem;font-weight:700;text-transform:uppercase;letter-spacing:.08em;margin-bottom:8px}.search input{width:100%;border:1px solid var(--line);background:var(--paper);border-radius:8px;padding:11px 12px;font:inherit;color:var(--ink);outline-color:var(--coral)}
nav section{margin:0 0 22px}nav h2{font-size:.72rem;color:var(--muted);text-transform:uppercase;letter-spacing:.12em;margin:0 0 8px}.nav-link{display:block;color:var(--ink);text-decoration:none;border-radius:8px;padding:7px 10px;margin:2px 0;font-size:.94rem}.nav-link:hover,.nav-link.active{background:#efe2d0;color:#0e423c}.nav-link.active{box-shadow:inset 3px 0 var(--coral)}
main{min-width:0;padding:34px clamp(22px,5vw,72px) 80px}.hero{display:flex;align-items:flex-start;justify-content:space-between;gap:22px;min-height:190px;border-bottom:1px solid var(--line);padding:36px 0 28px;position:relative}.hero:after{content:"";position:absolute;right:0;bottom:-1px;width:min(360px,45%);height:4px;background:linear-gradient(90deg,var(--coral),var(--ochre),var(--tide))}.eyebrow{margin:0 0 10px;color:var(--coral);font-weight:800;text-transform:uppercase;letter-spacing:.14em;font-size:.75rem}.hero h1{font-family:Fraunces,Georgia,serif;font-size:clamp(2.4rem,7vw,5.8rem);line-height:.9;letter-spacing:0;margin:0;max-width:900px}.repo{flex:0 0 auto;border:1px solid var(--ink);color:var(--ink);text-decoration:none;border-radius:8px;padding:9px 13px;font-weight:700;background:var(--paper)}
.doc{width:100%;max-width:920px;margin:42px 0 0;background:rgba(255,251,244,.72);box-shadow:var(--shadow);border:1px solid rgba(223,210,192,.82);border-radius:8px;padding:clamp(24px,4vw,54px);overflow-wrap:break-word}.doc h1{display:none}.doc h2{font-family:Fraunces,Georgia,serif;font-size:2rem;line-height:1.05;margin:2.2em 0 .5em}.doc h3{font-size:1.2rem;margin:1.8em 0 .35em}.doc h4{font-size:1rem;margin:1.4em 0 .2em;color:var(--reef)}.doc p{margin:0 0 1.05em}.doc ul,.doc ol{padding-left:1.35rem;margin:0 0 1.2em}.doc li{margin:.25em 0}.doc code{font-family:"IBM Plex Mono",ui-monospace,monospace;font-size:.9em;background:#efe2d0;border:1px solid #e2d2bd;border-radius:6px;padding:.08em .32em}.doc pre{overflow:auto;background:#12211f;color:#f8efe4;border-radius:8px;padding:18px 20px;border:1px solid #0b1715;box-shadow:inset 0 0 0 1px rgba(255,255,255,.04);margin:1.35em 0}.doc pre code{background:transparent;border:0;color:inherit;padding:0}.doc blockquote{margin:1.4em 0;padding:12px 16px;border-left:4px solid var(--coral);background:#f0e3d3;border-radius:0 8px 8px 0}.doc table{width:100%;border-collapse:collapse}.doc th,.doc td{border-bottom:1px solid var(--line);padding:8px;text-align:left}
@media(max-width:900px){.shell{display:block}.sidebar{position:relative;height:auto;max-height:270px;overflow:auto;border-right:0;border-bottom:1px solid var(--line);box-shadow:0 16px 40px rgba(18,33,31,.08)}.brand{margin-bottom:16px}.search{margin-bottom:16px}nav{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px 18px}nav section{margin:0}.nav-link{padding:6px 8px;font-size:.9rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}main{width:100vw;max-width:100vw;padding:20px;overflow:hidden}.hero{display:block;min-height:0;padding-top:24px}.repo{display:none}.doc{width:min(calc(100vw - 40px),350px);max-width:min(calc(100vw - 40px),350px);margin-top:24px;padding:22px}.doc p,.doc li{max-width:306px}.hero h1{font-size:clamp(2.25rem,11vw,2.9rem);max-width:100%;white-space:normal;word-break:break-word}}`;
}

function js() {
  return `
const input=document.getElementById('doc-search');
input?.addEventListener('input',()=>{const q=input.value.trim().toLowerCase();document.querySelectorAll('.nav-link').forEach(a=>{a.style.display=!q||a.textContent.toLowerCase().includes(q)?'block':'none'})});`;
}

function crabSvg() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120" role="img" aria-label="Crabbox">
<rect width="120" height="120" rx="24" fill="#12211f"/>
<path d="M24 60c9-26 62-26 72 0 3 9-4 28-36 28S21 69 24 60Z" fill="#e35e46"/>
<path d="M38 55c4-8 12-13 22-13s18 5 22 13" fill="none" stroke="#fffbf4" stroke-width="6" stroke-linecap="round"/>
<circle cx="48" cy="62" r="5" fill="#12211f"/><circle cx="72" cy="62" r="5" fill="#12211f"/>
<path d="M27 54 11 42m82 12 16-12M36 82 22 96m62-14 14 14M46 86l-5 17m33-17 5 17" stroke="#fffbf4" stroke-width="7" stroke-linecap="round"/>
<path d="M20 35c-4-13 8-22 18-14-10 2-13 9-18 14Zm80 0c4-13-8-22-18-14 10 2 13 9 18 14Z" fill="#e35e46"/>
</svg>`;
}

function slug(text) {
  return text.toLowerCase().replace(/`/g, "").replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
}

function escapeAttr(value) {
  return escapeHtml(value);
}
