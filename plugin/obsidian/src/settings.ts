import { App, PluginSettingTab, Setting, Notice } from "obsidian";
import type EngramBrainPlugin from "./main";

// ─── Settings Interface ────────────────────────────────────────────────────────

export interface EngramSettings {
	/** Base URL of the qbit-jay server. Default: http://127.0.0.1:7437 */
	engramUrl: string;
	/** Auto-sync interval in minutes. 0 = manual only. */
	autoSyncMinutes: number;
	/** Project name filter. Empty string = sync all projects. */
	projectFilter: string;
	/** Vault subfolder where notes are written. Default: "qbit-jay". */
	vaultSubfolder: string;
	/** ISO timestamp of the last successful sync. Empty = never synced. */
	lastSyncAt: string;
	/** Number of notes written on the last successful sync. */
	lastSyncCount: number;
}

export const DEFAULT_SETTINGS: EngramSettings = {
	engramUrl: "http://127.0.0.1:7437",
	autoSyncMinutes: 0,
	projectFilter: "",
	vaultSubfolder: "qbit-jay",
	lastSyncAt: "",
	lastSyncCount: 0,
};

// ─── Settings Tab ─────────────────────────────────────────────────────────────

export class EngramSettingTab extends PluginSettingTab {
	plugin: EngramBrainPlugin;

	constructor(app: App, plugin: EngramBrainPlugin) {
		super(app, plugin);
		this.plugin = plugin;
	}

	display(): void {
		const { containerEl } = this;
		containerEl.empty();

		containerEl.createEl("h2", { text: "qbit-jay" });
		containerEl.createEl("p", {
			text: "Sync your qbit-jay persistent memory into this vault as interconnected markdown notes.",
			cls: "setting-item-description",
		});

		// ── Engram URL ──────────────────────────────────────────────────────────
		new Setting(containerEl)
			.setName("qbit-jay URL")
			.setDesc(
				"Base URL of the running qbit-jay server. Must be reachable from Obsidian."
			)
			.addText((text) => {
				text
					.setPlaceholder("http://127.0.0.1:7437")
					.setValue(this.plugin.settings.engramUrl)
					.onChange(async (value) => {
						this.plugin.settings.engramUrl = value.trim();
						await this.plugin.saveSettings();
					});
				text.inputEl.style.width = "300px";
			})
			.addButton((button) => {
				button
					.setButtonText("Test Connection")
					.setTooltip("Verify the qbit-jay server is reachable")
					.onClick(async () => {
						const url = this.plugin.settings.engramUrl.trim();
						if (!url) {
							new Notice("qbit-jay URL is required");
							return;
						}
						try {
							const res = await fetch(`${url}/health`, {
								signal: AbortSignal.timeout(3000),
							});
							if (res.ok) {
								new Notice("✓ Connected to qbit-jay server");
							} else {
								new Notice(
									`Connection failed: server returned ${res.status}`
								);
							}
						} catch {
							new Notice(
								"Sync failed: could not reach qbit-jay server"
							);
						}
					});
			});

		// ── Auto-sync Interval ──────────────────────────────────────────────────
		new Setting(containerEl)
			.setName("Auto-sync interval")
			.setDesc(
				"How often to automatically sync. Set to 0 to disable automatic sync (manual only)."
			)
			.addDropdown((dropdown) => {
				dropdown
					.addOption("0", "Disabled (manual only)")
					.addOption("5", "Every 5 minutes")
					.addOption("15", "Every 15 minutes")
					.addOption("30", "Every 30 minutes")
					.addOption("60", "Every hour")
					.setValue(String(this.plugin.settings.autoSyncMinutes))
					.onChange(async (value) => {
						const minutes = parseInt(value, 10);
						this.plugin.settings.autoSyncMinutes = minutes;
						await this.plugin.saveSettings();
						// Immediately restart or clear the polling interval
						this.plugin.restartAutoSync();
					});
			});

		// ── Project Filter ──────────────────────────────────────────────────────
		new Setting(containerEl)
			.setName("Project filter")
			.setDesc(
				"Only sync observations from this project. Leave empty to sync all projects."
			)
			.addText((text) => {
				text
					.setPlaceholder("e.g. qbit-jay or my-project")
					.setValue(this.plugin.settings.projectFilter)
					.onChange(async (value) => {
						this.plugin.settings.projectFilter = value.trim();
						await this.plugin.saveSettings();
					});
			});

		// ── Vault Subfolder ─────────────────────────────────────────────────────
		new Setting(containerEl)
			.setName("Vault subfolder")
			.setDesc(
				"Folder inside this vault where qbit-jay notes are written. Never touches files outside this folder."
			)
			.addText((text) => {
				text
					.setPlaceholder("qbit-jay")
					.setValue(this.plugin.settings.vaultSubfolder)
					.onChange(async (value) => {
						const folder = value.trim() || "qbit-jay";
						this.plugin.settings.vaultSubfolder = folder;
						await this.plugin.saveSettings();
					});
			});

		// ── Manual Sync Button ──────────────────────────────────────────────────
		containerEl.createEl("h3", { text: "Sync" });

		const lastSyncEl = containerEl.createEl("p", {
			cls: "setting-item-description",
		});
		this.updateLastSyncText(lastSyncEl);

		new Setting(containerEl)
			.setName("Sync now")
			.setDesc("Manually trigger a sync with the qbit-jay server.")
			.addButton((button) => {
				button
					.setButtonText("Sync")
					.setCta()
					.onClick(async () => {
						button.setButtonText("Syncing…");
						button.setDisabled(true);
						await this.plugin.syncNow();
						button.setButtonText("Sync");
						button.setDisabled(false);
						this.updateLastSyncText(lastSyncEl);
					});
			});
	}

	private updateLastSyncText(el: HTMLElement): void {
		const { lastSyncAt, lastSyncCount } = this.plugin.settings;
		if (!lastSyncAt) {
			el.setText("Never synced.");
			return;
		}
		const date = new Date(lastSyncAt);
		const relative = formatRelative(date);
		el.setText(`Last sync: ${relative} · ${lastSyncCount} notes`);
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

/** Format a Date as a human-readable relative time string. */
export function formatRelative(date: Date): string {
	const diffMs = Date.now() - date.getTime();
	const diffSec = Math.floor(diffMs / 1000);

	if (diffSec < 5) return "just now";
	if (diffSec < 60) return `${diffSec}s ago`;

	const diffMin = Math.floor(diffSec / 60);
	if (diffMin < 60) return `${diffMin} min ago`;

	const diffHr = Math.floor(diffMin / 60);
	if (diffHr < 24) return `${diffHr}h ago`;

	const diffDay = Math.floor(diffHr / 24);
	return `${diffDay}d ago`;
}
