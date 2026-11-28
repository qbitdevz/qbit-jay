#!/usr/bin/env node
const MESSAGE = "The qbit-jay Pi plugin is not distributed through npm; install it from a local checkout with `pi install <repo>/plugin/pi`.";
const HELP = `pi-qbit-jay

Usage:
  pi-qbit-jay init

The qbit-jay Pi plugin is not distributed through npm, so init does not
install or pin any package and does not read or change Pi settings.
Install the plugin from a local checkout with:
pi install <repo>/plugin/pi
`;

if (process.argv[2] === "init") {
  console.error(MESSAGE);
  process.exitCode = 1;
} else {
  console.log(HELP);
}
