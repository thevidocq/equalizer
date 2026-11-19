// The in-panel Help is the manuals in docs/manual, one source for both.
// This copies them into the panel (screenshot links pointed at /help-img/)
// with the screenshots they use. --check fails when the copy is stale (CI).
import { copyFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const ui = new URL("..", import.meta.url).pathname;
const docs = join(ui, "../docs");
const content = join(ui, "src/features/help/content");
const images = join(ui, "public/help-img");
const check = process.argv.includes("--check");
const stale = [];

mkdirSync(content, { recursive: true });
mkdirSync(images, { recursive: true });
for (const guide of ["tenant", "admin"]) {
  const src = readFileSync(join(docs, "manual", `${guide}.md`), "utf8");
  const used = [...src.matchAll(/\(\.\.\/screenshots\/([a-z0-9-]+\.png)\)/g)].map((m) => m[1]);
  const out = src.replace(/\(\.\.\/screenshots\/([a-z0-9-]+\.png)\)/g, "(/help-img/$1)");
  const dest = join(content, `${guide}.md`);
  if (!existsSync(dest) || readFileSync(dest, "utf8") !== out) {
    if (check) stale.push(dest);
    else writeFileSync(dest, out);
  }
  for (const name of used) {
    const from = join(docs, "screenshots", name);
    const to = join(images, name);
    if (!existsSync(to) || !readFileSync(to).equals(readFileSync(from))) {
      if (check) stale.push(to);
      else copyFileSync(from, to);
    }
  }
}
if (stale.length) {
  console.error("Help is out of date with docs/manual (run npm run sync:help):\n" + stale.join("\n"));
  process.exit(1);
}
console.log(check ? "help ok" : "help synced");
