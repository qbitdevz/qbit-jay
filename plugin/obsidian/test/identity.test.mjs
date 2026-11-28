// qbit-jay identity (T0.4 S7, D3): the Obsidian plugin id is `qbit-jay` and the UI
// names qbit-jay, while saved settings keep their keys.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const read = (path) => readFileSync(new URL(path, import.meta.url), "utf8");
const manifest = JSON.parse(read("../manifest.json"));
const pkg = JSON.parse(read("../package.json"));

test("manifest id is qbit-jay (D3) and points at qbitdevz", () => {
  assert.equal(manifest.id, "qbit-jay");
  assert.equal(manifest.name, "qbit-jay");
  assert.equal(manifest.author, "qbitdevz");
  assert.equal(manifest.authorUrl, "https://github.com/qbitdevz/qbit-jay");
  assert.match(manifest.description, /qbit-jay/);
  assert.doesNotMatch(manifest.description, /Engram/);
});

test("package identity is qbit-jay", () => {
  assert.equal(pkg.name, "qbit-jay-obsidian");
  assert.ok(pkg.keywords.includes("qbit-jay"));
});

test("user-visible strings say qbit-jay, not Engram", () => {
  const visible = [];
  for (const file of ["../src/main.ts", "../src/settings.ts", "../src/sync.ts"]) {
    for (const match of read(file).matchAll(/(?:Notice\(|setName\(|setDesc\(|setText\(|setTooltip\(|setPlaceholder\(|text:|name:|new Error\()\s*[`"]([^`"]*)[`"]/g)) {
      visible.push(`${file}: ${match[1]}`);
    }
  }
  assert.ok(visible.length > 10, "the scan must find the UI strings");
  const offenders = visible.filter((line) => /engram/i.test(line.split(": ").slice(1).join(": ")) && !/e\.g\./.test(line));
  assert.deepEqual(offenders, []);
});

test("the default vault subfolder is qbit-jay", () => {
  assert.match(read("../src/settings.ts"), /vaultSubfolder: "qbit-jay"/);
});
