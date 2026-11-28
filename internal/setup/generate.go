package setup

// Sync embedded plugin copies from the source of truth (plugin/ directory).
// OpenCode and Pi are embedded — Claude Code is installed via marketplace.
// Run: go generate ./internal/setup/
//go:generate sh -c "rm -rf plugins/opencode && mkdir -p plugins/opencode && cp ../../plugin/opencode/qbit-jay.ts plugins/opencode/"
//go:generate sh -c "rm -rf plugins/pi && mkdir -p plugins/pi/assets && cd ../../plugin/pi && cp README.md cli.js compaction-recovery.js index.ts mcp-template.json memory-tool-chrome.js package.json private-redaction.js runtime-contract.js runtime-contract.json ../../internal/setup/plugins/pi/ && cp assets/* ../../internal/setup/plugins/pi/assets/"
