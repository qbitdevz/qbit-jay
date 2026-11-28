// Spec S5 (T5b): QBIT_JAY_* wins over ENGRAM_*, an empty QBIT_JAY_* value falls back to a set
// ENGRAM_* value, and a variable whose only values are empty still counts as set (L10). Every case
// observes what the extension does: the endpoint and headers it uses, the startup policy it
// enforces, whether it injects the protocol, or the error it reports.
import assert from "node:assert/strict";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { runtimeMetadataResponse } from "./support/runtime-contract-fixture.mjs";
import { importPluginFromSandbox, withPluginSandbox } from "./plugin-sandbox.mjs";

const LOCAL_ID = "0".repeat(32);
const DISABLED_NOTICE = "Engram memory is disabled by startup policy. Back up your existing Engram data, update Engram, restart its server and Pi, then recheck memory safety.";

// A preload that answers `instance-id` and `version` and then exits, so local mode never needs a
// real binary. The plugin runs `process.execPath instance-id` with this preload.
const PRELOAD = `
const path = require("node:path");
const command = path.basename(process.argv[1] || "");
if (command === "instance-id") { process.stdout.write("${LOCAL_ID}\\n"); process.exit(0); }
if (command === "version") { process.stdout.write("2.0.0-fixture\\n"); process.exit(0); }
`;

async function withEnvironment(env, run) {
  const saved = { ...process.env };
  const dir = await mkdtemp(join(tmpdir(), "qbit-jay-pi-env-"));
  const previousFetch = globalThis.fetch;
  try {
    for (const key of Object.keys(process.env)) {
      if (key.startsWith("QBIT_JAY_") || key.startsWith("ENGRAM_") || key === "GENTLE_PI_AGENTS_CHILD") delete process.env[key];
    }
    const preload = join(dir, "preload.cjs");
    await writeFile(preload, PRELOAD);
    process.env.NODE_OPTIONS = [saved.NODE_OPTIONS, `--require "${preload.replaceAll("\\", "\\\\")}"`].filter(Boolean).join(" ");
    process.env.QBIT_JAY_BIN = process.execPath;
    for (const [key, value] of Object.entries(env)) {
      if (value === undefined) delete process.env[key]; else process.env[key] = value;
    }
    return await run(dir);
  } finally {
    globalThis.fetch = previousFetch;
    for (const key of Object.keys(process.env)) if (!(key in saved)) delete process.env[key];
    Object.assign(process.env, saved);
    await rm(dir, { recursive: true, force: true });
  }
}

function installFetch(calls) {
  globalThis.fetch = async (url, init = {}) => {
    const parsed = new URL(url);
    calls.push({ origin: parsed.origin, path: parsed.pathname, headers: new Headers(init.headers) });
    if (parsed.pathname === "/runtime-contract") return runtimeMetadataResponse(parsed.pathname);
    if (parsed.pathname === "/health") return Response.json({ status: "ok", instance_id: LOCAL_ID });
    if (parsed.pathname === "/project/current") return Response.json({ project: "p" });
    if (parsed.pathname === "/topic-keys/suggest") return Response.json({ topic_key: "architecture/auth" });
    if (parsed.pathname === "/sessions") return Response.json({ id: "env-host", status: "created" });
    return Response.json({});
  };
}

// Loads the extension under `env` and runs one HTTP-backed tool call.
async function toolCall(env) {
  return withEnvironment(env, () => withPluginSandbox("qbit-jay-pi-env-", async ({ sandbox }) => {
    const calls = [];
    installFetch(calls);
    const tools = new Map();
    const register = await importPluginFromSandbox(sandbox);
    register({ registerTool(tool) { tools.set(tool.name, tool); }, on() {} });
    const ctx = { cwd: sandbox, sessionManager: { getSessionId: () => "env-host" }, ui: { setStatus() {} } };
    const result = await tools.get("mem_suggest_topic_key").execute("env", { title: "Auth" }, undefined, undefined, ctx);
    return { result, calls, text: JSON.stringify(result) };
  }));
}

const cases = (name, newValue, legacyValue, defaultValue) => {
  const n = `QBIT_JAY_${name}`, l = `ENGRAM_${name}`;
  return [
    ["new only", { [n]: newValue }, newValue],
    ["legacy only", { [l]: legacyValue }, legacyValue],
    ["both, new wins", { [n]: newValue, [l]: legacyValue }, newValue],
    ["new empty, legacy set", { [n]: "", [l]: legacyValue }, legacyValue],
    ["new blank, legacy set", { [n]: "  ", [l]: legacyValue }, legacyValue],
    ["new set, legacy empty", { [n]: newValue, [l]: "" }, newValue],
    ["both empty", { [n]: "", [l]: "" }, defaultValue],
    ["neither", {}, defaultValue],
  ];
};

function origins(calls) {
  return [...new Set(calls.filter((call) => call.path !== "/runtime-contract").map((call) => call.origin))];
}

for (const [name, env, want] of cases("URL", "http://127.0.0.1:17001", "http://127.0.0.1:17002", "http://127.0.0.1:7437")) {
  test(`URL: ${name}`, async () => {
    const { calls, result } = await toolCall(env);
    assert.notEqual(result.isError, true, JSON.stringify(result));
    assert.deepEqual(origins(calls), [want]);
  });
}

for (const [name, env, want] of cases("PORT", "17101", "17102", "7437")) {
  test(`PORT: ${name}`, async () => {
    const { calls, result } = await toolCall(env);
    assert.notEqual(result.isError, true, JSON.stringify(result));
    assert.deepEqual(origins(calls), [`http://127.0.0.1:${want}`]);
  });
}

test("PORT never overrides an explicit URL, under either name", async () => {
  for (const env of [
    { QBIT_JAY_URL: "http://127.0.0.1:17001", QBIT_JAY_PORT: "17101" },
    { ENGRAM_URL: "http://127.0.0.1:17001", QBIT_JAY_PORT: "17101" },
    { QBIT_JAY_URL: "http://127.0.0.1:17001", ENGRAM_PORT: "17102" },
  ]) {
    const { calls } = await toolCall(env);
    assert.deepEqual(origins(calls), ["http://127.0.0.1:17001"], JSON.stringify(env));
  }
});

// The bearer credential is opaque: a whitespace-only QBIT_JAY_HTTP_TOKEN is still a value and is
// not replaced by ENGRAM_HTTP_TOKEN (Headers trims it to a bare "Bearer").
for (const [name, extra, want] of [
  ["new only", { QBIT_JAY_HTTP_TOKEN: "new-token" }, "Bearer new-token"],
  ["legacy only", { ENGRAM_HTTP_TOKEN: "legacy-token" }, "Bearer legacy-token"],
  ["both, new wins", { QBIT_JAY_HTTP_TOKEN: "new-token", ENGRAM_HTTP_TOKEN: "legacy-token" }, "Bearer new-token"],
  ["new empty, legacy set", { QBIT_JAY_HTTP_TOKEN: "", ENGRAM_HTTP_TOKEN: "legacy-token" }, "Bearer legacy-token"],
  ["new set, legacy empty", { QBIT_JAY_HTTP_TOKEN: "new-token", ENGRAM_HTTP_TOKEN: "" }, "Bearer new-token"],
  ["new whitespace, legacy set", { QBIT_JAY_HTTP_TOKEN: " ", ENGRAM_HTTP_TOKEN: "legacy-token" }, "Bearer"],
  ["both empty", { QBIT_JAY_HTTP_TOKEN: "", ENGRAM_HTTP_TOKEN: "" }, null],
  ["neither", {}, null],
]) {
  test(`HTTP_TOKEN: ${name}`, async () => {
    const { calls, result } = await toolCall({ QBIT_JAY_URL: "http://127.0.0.1:17001", ...extra });
    assert.notEqual(result.isError, true, JSON.stringify(result));
    assert.ok(calls.length > 0);
    for (const call of calls) assert.equal(call.headers.get("Authorization"), want);
  });
}

// SERVER_MODE has three observable outcomes: unset runs shared mode, the literal
// session-owned conflicts with the explicit URL below, anything else is rejected.
const MODE_SHARED = "shared mode", MODE_OWNED = "owned conflict", MODE_INVALID = "invalid mode";
for (const [name, extra, want] of [
  ["new only", { QBIT_JAY_SERVER_MODE: "session-owned" }, MODE_OWNED],
  ["legacy only", { ENGRAM_SERVER_MODE: "session-owned" }, MODE_OWNED],
  ["both, new wins", { QBIT_JAY_SERVER_MODE: "session-owned", ENGRAM_SERVER_MODE: "bogus" }, MODE_OWNED],
  ["both, new invalid wins", { QBIT_JAY_SERVER_MODE: "bogus", ENGRAM_SERVER_MODE: "session-owned" }, MODE_INVALID],
  ["new empty, legacy set", { QBIT_JAY_SERVER_MODE: "", ENGRAM_SERVER_MODE: "session-owned" }, MODE_OWNED],
  ["new empty only is set, so invalid", { QBIT_JAY_SERVER_MODE: "" }, MODE_INVALID],
  ["both empty is set, so invalid", { QBIT_JAY_SERVER_MODE: "", ENGRAM_SERVER_MODE: "" }, MODE_INVALID],
  ["new whitespace is a value, so invalid", { QBIT_JAY_SERVER_MODE: " ", ENGRAM_SERVER_MODE: "session-owned" }, MODE_INVALID],
  ["neither", {}, MODE_SHARED],
]) {
  test(`SERVER_MODE: ${name}`, async () => {
    const { result, calls, text } = await toolCall({ QBIT_JAY_URL: "http://127.0.0.1:17001", ...extra });
    if (want === MODE_SHARED) {
      assert.notEqual(result.isError, true, text);
      return;
    }
    assert.equal(result.isError, true, text);
    assert.deepEqual(calls, []);
    if (want === MODE_OWNED) assert.match(text, /Session-owned qbit-jay conflicts with QBIT_JAY_URL \/ ENGRAM_URL/);
    else assert.match(text, /Invalid QBIT_JAY_SERVER_MODE \(or ENGRAM_SERVER_MODE\)/);
  });
}

// Owned mode owns its transport: any URL, PORT or SOCKET variable conflicts, even an empty one, and
// the empty QBIT_JAY_* value is a conflict on its own (an empty value is set, README "even empty
// ones"). Only when neither name is set does startup reach the owned child.
for (const key of ["URL", "PORT", "SOCKET"]) {
  for (const [name, env, conflicts] of [
    ["new only", { [`QBIT_JAY_${key}`]: "x" }, true],
    ["legacy only", { [`ENGRAM_${key}`]: "x" }, true],
    ["both", { [`QBIT_JAY_${key}`]: "x", [`ENGRAM_${key}`]: "y" }, true],
    ["new empty only", { [`QBIT_JAY_${key}`]: "" }, true],
    ["legacy empty only", { [`ENGRAM_${key}`]: "" }, true],
    ["new empty, legacy set", { [`QBIT_JAY_${key}`]: "", [`ENGRAM_${key}`]: "x" }, true],
    ["neither", {}, false],
  ]) {
    test(`owned mode conflict ${key}: ${name}`, async () => {
      const { result, text } = await toolCall({ QBIT_JAY_SERVER_MODE: "session-owned", QBIT_JAY_BIN: join(tmpdir(), "qbit-jay-no-such-binary"), ...env });
      assert.equal(result.isError, true, text);
      const conflict = new RegExp(`Session-owned qbit-jay conflicts with QBIT_JAY_${key} / ENGRAM_${key}`).test(text);
      assert.equal(conflict, conflicts, text);
    });
  }
}

// Disabled startup policy registers no tools and only warns at session start. Every supplied value
// except literal "allow" fails closed, so an empty or blank value that is not shadowing anything
// disables memory too.
async function startupPolicy(env) {
  return withEnvironment({ QBIT_JAY_URL: "http://127.0.0.1:17001", ...env }, () => withPluginSandbox("qbit-jay-pi-env-", async ({ sandbox }) => {
    const calls = [];
    installFetch(calls);
    const tools = new Map(), hooks = new Map(), notices = [];
    const register = await importPluginFromSandbox(sandbox);
    register({ registerTool(tool) { tools.set(tool.name, tool); }, on(event, handler) { hooks.set(event, handler); } });
    await hooks.get("session_start")({}, { hasUI: true, cwd: sandbox, sessionManager: { getSessionId: () => "env-host" }, ui: { setStatus() {}, notify(message) { notices.push(message); } } });
    return { disabled: tools.size === 0, notices, calls };
  }));
}

for (const [name, env, disabled] of [
  ["new disabled", { QBIT_JAY_STARTUP_POLICY: "disabled" }, true],
  ["legacy disabled", { ENGRAM_STARTUP_POLICY: "disabled" }, true],
  ["new disabled overrides legacy allow", { QBIT_JAY_STARTUP_POLICY: "disabled", ENGRAM_STARTUP_POLICY: "allow" }, true],
  ["new allow overrides legacy disabled", { QBIT_JAY_STARTUP_POLICY: "allow", ENGRAM_STARTUP_POLICY: "disabled" }, false],
  ["new only allow", { QBIT_JAY_STARTUP_POLICY: "allow" }, false],
  ["legacy only allow", { ENGRAM_STARTUP_POLICY: "allow" }, false],
  ["new empty falls back to legacy allow", { QBIT_JAY_STARTUP_POLICY: "", ENGRAM_STARTUP_POLICY: "allow" }, false],
  ["new empty falls back to legacy disabled", { QBIT_JAY_STARTUP_POLICY: "", ENGRAM_STARTUP_POLICY: "disabled" }, true],
  ["new empty only fails closed", { QBIT_JAY_STARTUP_POLICY: "" }, true],
  ["legacy empty only fails closed", { ENGRAM_STARTUP_POLICY: "" }, true],
  ["both empty fails closed", { QBIT_JAY_STARTUP_POLICY: "", ENGRAM_STARTUP_POLICY: "" }, true],
  ["new allow, legacy empty", { QBIT_JAY_STARTUP_POLICY: "allow", ENGRAM_STARTUP_POLICY: "" }, false],
  ["new whitespace is a value and fails closed", { QBIT_JAY_STARTUP_POLICY: " ", ENGRAM_STARTUP_POLICY: "allow" }, true],
  ["wrong case fails closed", { QBIT_JAY_STARTUP_POLICY: "ALLOW" }, true],
  ["neither", {}, false],
]) {
  test(`STARTUP_POLICY: ${name}`, async () => {
    const outcome = await startupPolicy(env);
    assert.equal(outcome.disabled, disabled);
    if (disabled) {
      assert.equal(outcome.notices.length, 1);
      assert.ok(outcome.notices[0].startsWith(DISABLED_NOTICE));
      assert.deepEqual(outcome.calls, [], "a disabled policy must not issue HTTP requests");
    }
  });
}

for (const [name, env, want] of [
  ["new only", { QBIT_JAY_STARTUP_NOTICE: "NEW" }, " NEW"],
  ["legacy only", { ENGRAM_STARTUP_NOTICE: "LEGACY" }, " LEGACY"],
  ["both, new wins", { QBIT_JAY_STARTUP_NOTICE: "NEW", ENGRAM_STARTUP_NOTICE: "LEGACY" }, " NEW"],
  ["new empty, legacy set", { QBIT_JAY_STARTUP_NOTICE: "", ENGRAM_STARTUP_NOTICE: "LEGACY" }, " LEGACY"],
  ["new set, legacy empty", { QBIT_JAY_STARTUP_NOTICE: "NEW", ENGRAM_STARTUP_NOTICE: "" }, " NEW"],
  ["new whitespace is a value", { QBIT_JAY_STARTUP_NOTICE: " ", ENGRAM_STARTUP_NOTICE: "LEGACY" }, "  "],
  ["both empty", { QBIT_JAY_STARTUP_NOTICE: "", ENGRAM_STARTUP_NOTICE: "" }, ""],
  ["neither", {}, ""],
]) {
  test(`STARTUP_NOTICE: ${name}`, async () => {
    const outcome = await startupPolicy({ QBIT_JAY_STARTUP_POLICY: "disabled", ...env });
    assert.deepEqual(outcome.notices, [DISABLED_NOTICE + want]);
  });
}

// DEFER_MEMORY defers the injected protocol only for the literal "1".
async function protocolInjected(env) {
  return withEnvironment({ QBIT_JAY_URL: "http://127.0.0.1:17001", ...env }, () => withPluginSandbox("qbit-jay-pi-env-", async ({ sandbox }) => {
    installFetch([]);
    const hooks = new Map();
    const register = await importPluginFromSandbox(sandbox);
    register({ registerTool() {}, on(event, handler) { hooks.set(event, handler); } });
    const ctx = { cwd: sandbox, sessionManager: { getSessionId: () => "env-host" }, ui: { setStatus() {} } };
    await hooks.get("session_start")({}, ctx);
    const result = await hooks.get("before_agent_start")({ systemPrompt: "base" }, ctx);
    return /Persistent Memory — Protocol/.test(result?.systemPrompt ?? "base");
  }));
}

for (const [name, env, injected] of [
  ["new 1 defers", { QBIT_JAY_DEFER_MEMORY: "1" }, false],
  ["legacy 1 defers", { ENGRAM_DEFER_MEMORY: "1" }, false],
  ["new 0 overrides legacy 1", { QBIT_JAY_DEFER_MEMORY: "0", ENGRAM_DEFER_MEMORY: "1" }, true],
  ["new 1 overrides legacy 0", { QBIT_JAY_DEFER_MEMORY: "1", ENGRAM_DEFER_MEMORY: "0" }, false],
  ["new empty falls back to legacy 1", { QBIT_JAY_DEFER_MEMORY: "", ENGRAM_DEFER_MEMORY: "1" }, false],
  ["new 1, legacy empty", { QBIT_JAY_DEFER_MEMORY: "1", ENGRAM_DEFER_MEMORY: "" }, false],
  ["new whitespace is a value, not 1", { QBIT_JAY_DEFER_MEMORY: " ", ENGRAM_DEFER_MEMORY: "1" }, true],
  ["both empty", { QBIT_JAY_DEFER_MEMORY: "", ENGRAM_DEFER_MEMORY: "" }, true],
  ["neither", {}, true],
]) {
  test(`DEFER_MEMORY: ${name}`, async () => {
    assert.equal(await protocolInjected(env), injected);
  });
}
