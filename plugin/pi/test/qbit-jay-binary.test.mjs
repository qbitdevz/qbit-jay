// qbit-jay identity (T0.4 S5, S9): the Pi package resolves the binary as
// QBIT_JAY_BIN, then ENGRAM_BIN (gentle-ai compatibility), then `qbit-jay` on PATH.
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { PLUGIN_ROOT, createPluginSandbox, importPluginFromSandbox } from "./plugin-sandbox.mjs";

const ENV_KEYS = ["QBIT_JAY_BIN", "ENGRAM_BIN", "QBIT_JAY_URL", "ENGRAM_URL", "QBIT_JAY_PORT", "ENGRAM_PORT", "QBIT_JAY_SERVER_MODE", "ENGRAM_SERVER_MODE", "QBIT_JAY_STARTUP_POLICY", "ENGRAM_STARTUP_POLICY"];

// Loads the plugin with the given environment and returns the binary-not-found
// message that the first memory tool call reports.
async function missingBinaryMessage(env) {
  const dir = await mkdtemp(join(tmpdir(), "qbit-jay-pi-bin-"));
  const savedPath = process.env.PATH;
  const saved = Object.fromEntries(ENV_KEYS.map((key) => [key, process.env[key]]));
  try {
    for (const key of ENV_KEYS) delete process.env[key];
    process.env.ENGRAM_PORT = "1"; // nothing listens: forces the spawn path
    Object.assign(process.env, env);
    const register = await importPluginFromSandbox(await createPluginSandbox(dir));
    const tools = new Map();
    register({ registerTool: (tool) => tools.set(tool.name, tool), on() {} });
    const ctx = { hasUI: false, cwd: dir, sessionManager: { getSessionId: () => "s" }, ui: { setStatus() {}, notify() {} } };
    const result = await tools.get("mem_save").execute("c", { content: "x" }, undefined, undefined, ctx);
    return result.content[0].text;
  } finally {
    for (const key of ENV_KEYS) {
      if (saved[key] === undefined) delete process.env[key];
      else process.env[key] = saved[key];
    }
    process.env.PATH = savedPath;
    await rm(dir, { recursive: true, force: true });
  }
}

test("QBIT_JAY_BIN wins over ENGRAM_BIN", async () => {
  const text = await missingBinaryMessage({ QBIT_JAY_BIN: "qj-bin-does-not-exist", ENGRAM_BIN: "legacy-bin-does-not-exist" });
  assert.match(text, /qj-bin-does-not-exist/);
  assert.doesNotMatch(text, /legacy-bin-does-not-exist/);
});

test("ENGRAM_BIN still works as the gentle-ai compatibility fallback", async () => {
  const text = await missingBinaryMessage({ ENGRAM_BIN: "legacy-bin-does-not-exist" });
  assert.match(text, /legacy-bin-does-not-exist/);
});

test("an empty QBIT_JAY_BIN does not mask ENGRAM_BIN", async () => {
  const text = await missingBinaryMessage({ QBIT_JAY_BIN: " ", ENGRAM_BIN: "legacy-bin-does-not-exist" });
  assert.match(text, /legacy-bin-does-not-exist/);
});

test("the default binary is qbit-jay from PATH and the hint names QBIT_JAY_BIN", async () => {
  const text = await missingBinaryMessage({ PATH: "" });
  assert.match(text, /"qbit-jay"/);
  assert.match(text, /QBIT_JAY_BIN/);
});

test("the MCP template starts qbit-jay, honoring QBIT_JAY_BIN then ENGRAM_BIN", async () => {
  const template = JSON.parse(await readFile(join(PLUGIN_ROOT, "mcp-template.json"), "utf8"));
  assert.deepEqual(Object.keys(template.mcpServers), ["qbit-jay"]);
  const script = template.mcpServers["qbit-jay"].args.join(" ");
  assert.match(script, /process\.env\.QBIT_JAY_BIN/);
  assert.match(script, /process\.env\.ENGRAM_BIN/);
  assert.match(script, /'qbit-jay'/);
});
