import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const packageMetadata = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8"));
const expectedRef = `refs/tags/pi-v${packageMetadata.version}`;

const releaseRef = process.argv[2];
const cliPath = fileURLToPath(new URL("../cli.js", import.meta.url));

if (releaseRef !== expectedRef) {
	throw new Error(`Pi release ref ${JSON.stringify(releaseRef)} must match package version tag ${expectedRef}`);
}

const help = execFileSync(process.execPath, [cliPath], { encoding: "utf8" });
if (!help.includes("pi install <repo>/plugin/pi") || help.includes("npm:")) {
	throw new Error("Pi CLI help must point to the local install and name no npm source");
}
