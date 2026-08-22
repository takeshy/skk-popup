// Builds frontend/public/dictionary.json from the dictionaries listed in
// dictionary_sources.json (ported from chrome-skk-lite's build_extension.js).
//
// Skips work when the output already exists; pass --force to regenerate.
const fs = require("node:fs");
const path = require("node:path");
const zlib = require("node:zlib");
const decoder = new TextDecoder("euc-jp");

const ROOT = path.resolve(__dirname, "..");
const BUILD_CONFIG_FILE = path.join(ROOT, "dictionary_sources.json");
const OUTPUT_FILE = path.join(ROOT, "frontend", "public", "dictionary.json");

function loadBuildConfig() {
  const config = JSON.parse(fs.readFileSync(BUILD_CONFIG_FILE, "utf8"));
  if (!config || typeof config !== "object") {
    throw new Error(`Invalid build config: ${BUILD_CONFIG_FILE}`);
  }
  if (!Array.isArray(config.dictionaries) || config.dictionaries.length === 0) {
    throw new Error("Build config must contain a non-empty dictionaries array.");
  }
  if (typeof config.sourceBaseUrl !== "string" || !config.sourceBaseUrl) {
    throw new Error("Build config must contain sourceBaseUrl.");
  }
  return config;
}

function dictionaryArchiveUrl(sourceBaseUrl, dictionaryName) {
  return `${sourceBaseUrl.replace(/\/+$/, "")}/${encodeURIComponent(dictionaryName)}.gz`;
}

async function fetchDictionaryText(sourceBaseUrl, dictionaryName) {
  const url = dictionaryArchiveUrl(sourceBaseUrl, dictionaryName);
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Failed to fetch ${dictionaryName} (${response.status} ${response.statusText})`);
  }
  const archiveBuffer = Buffer.from(await response.arrayBuffer());
  const textBuffer = zlib.gunzipSync(archiveBuffer);
  return decoder.decode(textBuffer);
}

function parseDictionary(text, dict) {
  const lines = text.split("\n");
  for (let line of lines) {
    line = line.trim();
    if (!line || line.startsWith(";;")) continue;

    const spaceIndex = line.indexOf(" ");
    if (spaceIndex === -1) continue;

    const kana = line.substring(0, spaceIndex);
    const rest = line.substring(spaceIndex + 1);
    if (!rest.startsWith("/") || !rest.endsWith("/")) continue;

    const entries = rest.substring(1, rest.length - 1).split("/");
    let bucket = dict[kana];
    if (!bucket) {
      bucket = [];
      dict[kana] = bucket;
    }

    for (const entry of entries) {
      if (!entry) continue;
      // Keep the annotation (";..." suffix); dedupe by the word before it.
      const word = entry.split(";")[0];
      if (word && !bucket.some((item) => item.split(";")[0] === word)) {
        bucket.push(entry);
      }
    }
  }
}

async function main() {
  if (process.argv.includes("--force")) {
    fs.rmSync(OUTPUT_FILE, { force: true });
  }
  if (fs.existsSync(OUTPUT_FILE)) {
    console.log(`Skipping dictionary build: ${OUTPUT_FILE} already exists (use --force to rebuild)`);
    return;
  }

  const buildConfig = loadBuildConfig();
  const dict = Object.create(null);

  for (const dictionaryName of buildConfig.dictionaries) {
    const beforeKeys = Object.keys(dict).length;
    const text = await fetchDictionaryText(buildConfig.sourceBaseUrl, dictionaryName);
    parseDictionary(text, dict);
    console.log(`Processed ${dictionaryName}: total ${Object.keys(dict).length} keys (+${Object.keys(dict).length - beforeKeys})`);
  }

  fs.mkdirSync(path.dirname(OUTPUT_FILE), { recursive: true });
  const dictionaryJson = JSON.stringify(dict);
  fs.writeFileSync(OUTPUT_FILE, dictionaryJson);

  console.log(`Wrote ${OUTPUT_FILE}`);
  console.log(`Total keys: ${Object.keys(dict).length}`);
  console.log(`Output size: ${Buffer.byteLength(dictionaryJson)} bytes`);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
