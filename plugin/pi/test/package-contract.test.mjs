import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

// Regression test for https://github.com/Gentleman-Programming/engram/issues/853
//
// qbit-jay runs inside pi and only ever imports `Text` from
// @earendil-works/pi-tui, yet declared it as a hard `^0.74.0` dependency. That gave
// npm a legal reason to hoist pi-tui 0.74.x to the root of `~/.pi/agent/npm` over the
// `^0.84.x` range declared by the installed pi-coding-agent, crashing every pi child
// spawn with `SyntaxError: ... does not provide an export named 'TuiMainScreen'`.
// Declaring pi-tui as an optional peer dependency lets the host's copy win and keeps
// npm from reintroducing the downgrade.

const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8"));
const mcpTemplate = JSON.parse(readFileSync(new URL("../mcp-template.json", import.meta.url), "utf8"));
const indexSource = readFileSync(new URL("../index.ts", import.meta.url), "utf8");

test("package distributes both runtime contract assets beside its explicit extension", () => {
	assert.deepEqual(pkg.pi.extensions, ["./index.ts"]);
	for (const asset of ["runtime-contract.js", "runtime-contract.json"]) {
		assert.ok(pkg.files.includes(asset), `${asset} must be included in the package files`);
		assert.ok(existsSync(new URL(`../${asset}`, import.meta.url)), `${asset} must exist beside index.ts`);
	}
});

test("package advertises startup policy and session-owned wire contracts v1", () => {
	assert.deepEqual(pkg.engram, { startupPolicy: 1, sessionOwned: 1 });
});

test("package test runner serializes files without dropping any public tests", () => {
	assert.equal(pkg.scripts.test, "node --test --test-concurrency=1 test/*.test.mjs");
});

const PI_TUI = "@earendil-works/pi-tui";
test("next Pi package release is 0.8.0", () => {
	assert.equal(pkg.version, "0.8.0");
});
const CLI_PATH = fileURLToPath(new URL("../cli.js", import.meta.url));
const RELEASE_CONTRACT_PATH = fileURLToPath(new URL("./release-contract.mjs", import.meta.url));

function runReleaseContract(tag) {
	return spawnSync(process.execPath, [RELEASE_CONTRACT_PATH, tag], { encoding: "utf8" });
}

/**
 * Minimal range check for the peer declaration contract. Supports the range forms the
 * repo actually uses for peer dependencies (`>=x.y.z` and `*`). Anything else throws so
 * a range change here is a conscious decision, not an accident.
 *
 * The reviewed contract (#853) is an OPEN-ENDED >= floor: the plugin only imports
 * `Text` while running inside pi, so it must accept every future pi-tui release
 * (0.85.x, 1.x, 2.x, ...) with no fix required here. The acceptance list below is a
 * floor verification against the two known lines, not a version pin.
 */
function peerRangeAllows(range, version) {
	if (range === "*") return true;
	const match = /^>=(\d+)\.(\d+)\.(\d+)$/.exec(range);
	if (!match) {
		throw new Error(
			`peer range ${JSON.stringify(range)} uses a form this contract test cannot evaluate. ` +
				"The reviewed contract (#853) is an open-ended >= floor, so new pi-tui releases " +
				"keep being accepted without any fix here. If the range change is deliberate " +
				"(e.g. adding an upper bound for a breaking pi-tui major), update peerRangeAllows() " +
				"and the acceptance list in this test consciously.",
		);
	}
	const [major, minor, patch] = version.split(".").map(Number);
	const [floorMajor, floorMinor, floorPatch] = match.slice(1).map(Number);
	if (major !== floorMajor) return major > floorMajor;
	if (minor !== floorMinor) return minor > floorMinor;
	return patch >= floorPatch;
}

// Regression contract for #1557: TypeBox is provided by the Pi host.
test("typebox is not a hard or optional dependency", () => {
	assert.equal(pkg.dependencies?.typebox, undefined, "the Pi host must provide typebox, not a hard dependency");
	assert.equal(pkg.optionalDependencies?.typebox, undefined, "optional dependencies would still install a separate typebox copy");
});

test("typebox is declared as a wildcard peer dependency", () => {
	assert.equal(pkg.peerDependencies?.typebox, "*", "typebox must accept the Pi host's version without a package-owned constraint");
});

test("typebox peer dependency is optional", () => {
	assert.equal(pkg.peerDependenciesMeta?.typebox?.optional, true, "npm must not auto-install a separate typebox peer");
});

test("pi-tui is not a hard dependency", () => {
	assert.equal(
		pkg.dependencies?.[PI_TUI],
		undefined,
		"pi-tui must not be declared in dependencies: a hard dep gives npm a reason to hoist a 0.74.x over the host's ^0.84.x and crash pi startup",
	);
	assert.equal(
		pkg.optionalDependencies?.[PI_TUI],
		undefined,
		"pi-tui must not be declared in optionalDependencies either: npm installs optional dependencies by default, so this door would reintroduce the same downgrade",
	);
});

test("pi-tui is declared as an optional peer dependency", () => {
	const range = pkg.peerDependencies?.[PI_TUI];
	assert.ok(range, "pi-tui must be declared in peerDependencies so the host pi installation's copy is used");
	assert.equal(
		pkg.peerDependenciesMeta?.[PI_TUI]?.optional,
		true,
		"the peer must be optional so npm never auto-installs a second pi-tui into pi-managed trees",
	);
});

test("pi-tui peer range accepts every pi-tui line the plugin renders against", () => {
	const range = pkg.peerDependencies?.[PI_TUI];
	assert.ok(range, "peerDependencies must declare a pi-tui range");
	// `Text` is the only export used and exists across the 0.74 -> 0.84 lines; a caret
	// range on 0.x would pin the minor and conflict with the host's 0.84.x.
	for (const version of ["0.74.0", "0.74.2", "0.84.3"]) {
		assert.ok(
			peerRangeAllows(range, version),
			`peer range ${JSON.stringify(range)} must accept pi-tui ${version} (host ships 0.84.x, plugin is verified against 0.74.x)`,
		);
	}
	// Pin the lower bound, not just acceptance: a wider range (`*`, `>=0.0.0`) would
	// silently bless pi-tui lines the plugin has never rendered against. Note `*` is
	// accepted by the helper but rejected here on purpose: `*` keeps accepting future
	// releases, yet it would also bless a 0.x line below the verified 0.74 floor.
	assert.equal(
		peerRangeAllows(range, "0.73.9"),
		false,
		`peer range ${JSON.stringify(range)} must reject pi-tui 0.73.9: the >=0.74.0 floor is the reviewed contract`,
	);
});

test("the peer declaration stays justified by actual pi-tui usage", () => {
	assert.match(
		indexSource,
		/from "@earendil-works\/pi-tui"/,
		"index.ts imports pi-tui; if that import is ever removed, drop the peer dependency too",
	);
});

test("pi-qbit-jay init refuses npm distribution and never touches Pi settings", () => {
	for (const original of [undefined, JSON.stringify({ packages: ["npm:gentle-engram@0.8.0", "npm:existing"] })]) {
		const agentDir = mkdtempSync(join(tmpdir(), "qbit-jay-pi-cli-"));
		try {
			if (original !== undefined) writeFileSync(join(agentDir, "settings.json"), original);
			const result = spawnSync(process.execPath, [CLI_PATH, "init"], {
				encoding: "utf8", env: { ...process.env, PI_CODING_AGENT_DIR: agentDir },
			});
			assert.notEqual(result.status, 0);
			assert.match(result.stderr, /not distributed through npm/);
			assert.match(result.stderr, /pi install <repo>\/plugin\/pi/);
			const outputs = result.stdout + result.stderr;
			assert.doesNotMatch(outputs, /npm:qbit-jay-pi|npm:gentle-engram/);
			assert.equal(existsSync(join(agentDir, "mcp.json")), false);
			if (original === undefined) assert.equal(existsSync(join(agentDir, "settings.json")), false);
			else assert.equal(readFileSync(join(agentDir, "settings.json"), "utf8"), original);
		} finally {
			rmSync(agentDir, { recursive: true, force: true });
		}
	}
});

test("pi-qbit-jay help names the local install and no registry source", () => {
	const help = execFileSync(process.execPath, [CLI_PATH], { encoding: "utf8" });
	assert.match(help, /pi install <repo>\/plugin\/pi/);
	assert.doesNotMatch(help, /npm:/);
});

test("Pi MCP template retains its opt-in launcher, QBIT_JAY_BIN first and the ENGRAM_BIN fallback", () => {
	const server = mcpTemplate.mcpServers["qbit-jay"];
	assert.deepEqual(
		{ command: server.command, lifecycle: server.lifecycle, directTools: server.directTools },
		{ command: "node", lifecycle: "lazy", directTools: false },
	);
	assert.equal(server.args.length, 2);
	assert.equal(server.args[0], "-e");
	assert.match(server.args[1], /spawn\(bin, \['mcp', '--tools=agent'\], \{ stdio: 'inherit' \}\)/);

	const launcherPrefix = server.args[1].slice(0, server.args[1].indexOf("const child = spawn"));
	assert.notEqual(launcherPrefix, server.args[1], "launcher must select a binary before spawning it");
	const selectBin = (env) => Function("require", "process", `${launcherPrefix}; return bin;`)(
		(moduleName) => {
			assert.equal(moduleName, "node:child_process");
			return { spawn() {} };
		},
		{ env },
	);

	assert.equal(selectBin({}), "qbit-jay", "absent binary variables fall back to qbit-jay");
	assert.equal(selectBin({ ENGRAM_BIN: "" }), "qbit-jay", "an empty ENGRAM_BIN falls back to qbit-jay");
	assert.equal(selectBin({ ENGRAM_BIN: " \t " }), "qbit-jay", "a whitespace-only ENGRAM_BIN falls back to qbit-jay");
	assert.equal(selectBin({ ENGRAM_BIN: "  /custom/bin  " }), "  /custom/bin  ", "a nonblank ENGRAM_BIN is preserved exactly");
	assert.equal(selectBin({ QBIT_JAY_BIN: "/qj", ENGRAM_BIN: "/legacy" }), "/qj", "QBIT_JAY_BIN wins over ENGRAM_BIN");
	assert.equal(selectBin({ QBIT_JAY_BIN: " ", ENGRAM_BIN: "/legacy" }), "/legacy", "a blank QBIT_JAY_BIN does not mask ENGRAM_BIN");
});

test("Pi release contract accepts the matching full tag ref and CLI guidance", () => {
	const result = runReleaseContract(`refs/tags/pi-v${pkg.version}`);
	assert.equal(result.status, 0, result.stderr);
});

test("Pi release contract rejects a tag that differs from the package version", () => {
	const result = runReleaseContract("refs/tags/pi-v0.0.0");
	assert.notEqual(result.status, 0);
	assert.match(result.stderr, /must match package version/);
});

test("Pi release contract rejects branch refs even when their names match", () => {
	for (const ref of ["refs/heads/main", `refs/heads/pi-v${pkg.version}`, `pi-v${pkg.version}`]) {
		const result = runReleaseContract(ref);
		assert.notEqual(result.status, 0, `${ref} must not be publishable`);
		assert.match(result.stderr, /must match package version/);
	}
});

