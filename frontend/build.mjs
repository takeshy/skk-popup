// Copies the static frontend into dist/ for go:embed. No bundler: the
// sources are plain JS/CSS and dictionary.json must not be parsed by a
// bundler anyway.
import { cpSync, existsSync, mkdirSync, readdirSync, rmSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const frontendDir = dirname(fileURLToPath(import.meta.url));
const distDir = join(frontendDir, "dist");

if (!existsSync(join(frontendDir, "public", "dictionary.json"))) {
  console.error("frontend/public/dictionary.json is missing. Run `npm run build` so prebuild can generate it first.");
  process.exit(1);
}

for (const entry of readdirSync(distDir)) {
  if (entry === ".gitkeep") continue;
  rmSync(join(distDir, entry), { recursive: true, force: true });
}
mkdirSync(distDir, { recursive: true });

cpSync(join(frontendDir, "index.html"), join(distDir, "index.html"));
cpSync(join(frontendDir, "src"), join(distDir, "src"), { recursive: true });
cpSync(
  join(frontendDir, "public", "dictionary.json"),
  join(distDir, "dictionary.json")
);

let totalBytes = 0;
const walk = (dir) => {
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry);
    if (statSync(p).isDirectory()) walk(p);
    else totalBytes += statSync(p).size;
  }
};
walk(distDir);
console.log(`Copied frontend to ${distDir} (${totalBytes} bytes)`);
