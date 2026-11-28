#!/usr/bin/env node
const MESSAGE = "The qbit-jay Pi plugin is embedded in the qbit-jay binary, not installed from npm. Run `qbit-jay setup pi` to install it.";
const HELP = `pi-qbit-jay

Usage:
  pi-qbit-jay init

The qbit-jay Pi plugin is embedded in the qbit-jay binary and is not published
to npm, so init installs nothing and does not read or change Pi settings.
Install it with:
qbit-jay setup pi
`;

if (process.argv[2] === "init") {
  console.error(MESSAGE);
  process.exitCode = 1;
} else {
  console.log(HELP);
}
