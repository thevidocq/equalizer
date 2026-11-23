// Enforces the panel-ui rules (CLAUDE.md):
// - shared/ never imports from features/ or app/;
// - a feature imports another feature only through its index.ts;
// - no file over 200 lines;
// - effects have a block body (whatever they return is called as cleanup).
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, sep } from "node:path";

const root = new URL("../src", import.meta.url).pathname;
const errors = [];

function walk(dir) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p);
    else if (/\.(ts|tsx)$/.test(name) && !name.endsWith(".d.ts")) check(p);
  }
}

function check(file) {
  const rel = relative(root, file).split(sep).join("/");
  const text = readFileSync(file, "utf8");
  const lines = text.split("\n").length;
  // An effect written as () => expression returns that value, and React calls
  // whatever an effect returns as its cleanup: a Promise (scrollIntoView in
  // recent browsers) then crashes the page. Effects take a block body.
  if (/use(Layout)?Effect\(\s*\(\)\s*=>\s*[^{\s]/.test(text)) errors.push(`${rel}: write the effect as () => { ... }`);
  if (lines > 200) errors.push(`${rel}: ${lines} lines (limit 200)`);
  const feature = rel.startsWith("features/") ? rel.split("/")[1] : null;
  for (const m of text.matchAll(/from\s+"(@\/[^"]+)"/g)) {
    const target = m[1].slice(2);
    if (rel.startsWith("shared/") && (target.startsWith("features/") || target.startsWith("app/"))) {
      errors.push(`${rel}: shared code imports ${m[1]}`);
    }
    if (target.startsWith("features/")) {
      const [, other, ...rest] = target.split("/");
      if (other !== feature && rest.length > 0) errors.push(`${rel}: import ${m[1]} through features/${other}/index.ts`);
    }
  }
}

walk(root);
if (errors.length) {
  console.error(errors.join("\n"));
  process.exit(1);
}
console.log("boundaries ok");
