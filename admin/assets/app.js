const ui = {};
const fileState = { root: "", path: "" };
const THEME_STORAGE_KEY = "twitcast-theme";
const HIDE_OFFLINE_LOGS_STORAGE_KEY = "twitcast-hide-offline-logs";
const themeState = { value: "dark" };
const logState = { lines: [], hideOffline: true, filteredCount: 0 };
let botRestartPending = false;
let toastTimer = null;

window.addEventListener("DOMContentLoaded", () => {
    cacheElements();
    initTheme();
    initLogFilters();
    bindEvents();
    boot().catch(handleError);
});

function cacheElements() {
    ui.statusBadge = document.getElementById("statusBadge");
    ui.recorderSummary = document.getElementById("recorderSummary");
    ui.activeRecordings = document.getElementById("activeRecordings");
    ui.runtimeInfo = document.getElementById("runtimeInfo");
    ui.diagnostics = document.getElementById("diagnostics");

    ui.langInput = document.getElementById("langInput");
    ui.enableLogInput = document.getElementById("enableLogInput");
    ui.streamersBody = document.getElementById("streamersBody");

    ui.discordEnabledInput = document.getElementById("discordEnabledInput");
    ui.discordTokenInput = document.getElementById("discordTokenInput");
    ui.discordGuildInput = document.getElementById("discordGuildInput");
    ui.discordNotifyInput = document.getElementById("discordNotifyInput");
    ui.discordArchiveInput = document.getElementById("discordArchiveInput");
    ui.discordTagRoleInput = document.getElementById("discordTagRoleInput");
    ui.discordTestBtn = document.getElementById("discordTestBtn");

    ui.telegramEnabledInput = document.getElementById("telegramEnabledInput");
    ui.telegramTokenInput = document.getElementById("telegramTokenInput");
    ui.telegramChatInput = document.getElementById("telegramChatInput");
    ui.telegramEndpointInput = document.getElementById("telegramEndpointInput");
    ui.telegramConvertInput = document.getElementById("telegramConvertInput");
    ui.telegramKeepInput = document.getElementById("telegramKeepInput");
    ui.telegramTestBtn = document.getElementById("telegramTestBtn");

    ui.fileRootSelect = document.getElementById("fileRootSelect");
    ui.filePathLabel = document.getElementById("filePathLabel");
    ui.fileUpBtn = document.getElementById("fileUpBtn");
    ui.filesBody = document.getElementById("filesBody");
    ui.logsPanel = document.getElementById("logsPanel");
    ui.logsSummary = document.getElementById("logsSummary");
    ui.hideOfflineLogsInput = document.getElementById("hideOfflineLogsInput");
    ui.checkVersionBtn = document.getElementById("checkVersionBtn");

    ui.themeToggleBtn = document.getElementById("themeToggleBtn");
    ui.refreshBtn = document.getElementById("refreshBtn");
    ui.saveSettingsBtn = document.getElementById("saveSettingsBtn");
    ui.startRecorderBtn = document.getElementById("startRecorderBtn");
    ui.stopRecorderBtn = document.getElementById("stopRecorderBtn");
    ui.restartRecorderBtn = document.getElementById("restartRecorderBtn");
    ui.restartBotBtn = document.getElementById("restartBotBtn");
    ui.addStreamerBtn = document.getElementById("addStreamerBtn");
    ui.fileRefreshBtn = document.getElementById("fileRefreshBtn");
    ui.logsRefreshBtn = document.getElementById("logsRefreshBtn");
    ui.toast = document.getElementById("toast");
}

function bindEvents() {
    ui.themeToggleBtn.addEventListener("click", toggleTheme);
    ui.refreshBtn.addEventListener("click", () => refreshAll().catch(handleError));
    ui.saveSettingsBtn.addEventListener("click", () => saveSettings().catch(handleError));
    ui.checkVersionBtn.addEventListener("click", () => checkForUpdates().catch(handleError));
    ui.discordTestBtn.addEventListener("click", () => sendDiscordTest().catch(handleError));
    ui.telegramTestBtn.addEventListener("click", () => sendTelegramTest().catch(handleError));
    ui.startRecorderBtn.addEventListener("click", () => controlRecorder("start").catch(handleError));
    ui.stopRecorderBtn.addEventListener("click", () => controlRecorder("stop").catch(handleError));
    ui.restartRecorderBtn.addEventListener("click", () => controlRecorder("restart").catch(handleError));
    ui.restartBotBtn.addEventListener("click", () => restartBot().catch(handleError));
    ui.addStreamerBtn.addEventListener("click", addStreamerRow);
    ui.fileRootSelect.addEventListener("change", () => browseFiles(ui.fileRootSelect.value, "").catch(handleError));
    ui.fileUpBtn.addEventListener("click", () => browseFiles(fileState.root, parentPath(fileState.path)).catch(handleError));
    ui.fileRefreshBtn.addEventListener("click", () => browseFiles(fileState.root, fileState.path).catch(handleError));
    ui.logsRefreshBtn.addEventListener("click", () => loadLogs().catch(handleError));
    ui.hideOfflineLogsInput.addEventListener("change", handleOfflineLogFilterChange);
}

// Theme needs to be applied before the first full paint to avoid a flash of the wrong palette.
function initTheme() {
    applyTheme(readStoredTheme());
}

function toggleTheme() {
    applyTheme(themeState.value === "dark" ? "light" : "dark");
}

function readStoredTheme() {
    try {
        return normalizeTheme(window.localStorage.getItem(THEME_STORAGE_KEY));
    } catch {
        return "dark";
    }
}

function normalizeTheme(theme) {
    return theme === "light" ? "light" : "dark";
}

function applyTheme(theme) {
    const nextTheme = normalizeTheme(theme);
    themeState.value = nextTheme;
    document.documentElement.dataset.theme = nextTheme;

    if (ui.themeToggleBtn) {
        const isDark = nextTheme === "dark";
        ui.themeToggleBtn.textContent = isDark ? "Use Light Theme" : "Use Dark Theme";
        ui.themeToggleBtn.setAttribute("aria-label", isDark ? "Switch to light theme" : "Switch to dark theme");
        ui.themeToggleBtn.setAttribute("aria-pressed", String(isDark));
    }

    try {
        window.localStorage.setItem(THEME_STORAGE_KEY, nextTheme);
    } catch {}
}

function initLogFilters() {
    logState.hideOffline = readStoredHideOfflineLogs();
    ui.hideOfflineLogsInput.checked = logState.hideOffline;
}

function readStoredHideOfflineLogs() {
    try {
        const stored = window.localStorage.getItem(HIDE_OFFLINE_LOGS_STORAGE_KEY);
        return stored !== "0";
    } catch {
        return true;
    }
}

function handleOfflineLogFilterChange() {
    logState.hideOffline = ui.hideOfflineLogsInput.checked;
    try {
        window.localStorage.setItem(HIDE_OFFLINE_LOGS_STORAGE_KEY, logState.hideOffline ? "1" : "0");
    } catch {}
    loadLogs().catch(handleError);
}

async function boot() {
    await refreshAll();
    window.setInterval(() => loadStatus().catch(() => {}), 5000);
    window.setInterval(() => loadLogs().catch(() => {}), 5000);
}

async function refreshAll() {
    await Promise.all([loadStatus(), loadSettings(), loadLogs()]);
}

async function api(url, options = {}) {
    const response = await fetch(url, {
        headers: {
            "Content-Type": "application/json",
            ...(options.headers || {})
        },
        ...options
    });

    const contentType = response.headers.get("content-type") || "";
    const payload = contentType.includes("application/json") ? await response.json() : null;
    if (!response.ok) {
        throw new Error(payload?.error || response.statusText || "Request failed");
    }
    return payload;
}

async function loadStatus() {
    const data = await api("/api/status");
    renderStatus(data);
    renderFileRoots(data.file_roots || []);

    if (!fileState.root && data.file_roots?.length) {
        await browseFiles(data.file_roots[0].root, "");
    }
}

async function loadSettings() {
    const settings = await api("/api/settings");
    renderSettings(settings);

    if (!fileState.root) {
        const roots = await api("/api/files/roots");
        renderFileRoots(roots);
        if (roots.length) {
            await browseFiles(roots[0].root, "");
        }
    }
}

async function loadLogs() {
    const params = new URLSearchParams({ limit: "250" });
    if (logState.hideOffline) {
        params.set("hide_offline", "1");
    }
    const data = await api(`/api/logs?${params.toString()}`);
    logState.lines = Array.isArray(data.lines) ? data.lines : [];
    logState.filteredCount = Number.isFinite(data.filtered_count) ? data.filtered_count : 0;
    renderLogs();
}

// Filter offline polling on the backend first so noisy lines do not push useful logs out of the latest window.
function renderLogs() {
    ui.logsSummary.textContent = buildLogSummary();

    if (!logState.lines.length) {
        if (logState.hideOffline && logState.filteredCount > 0) {
            ui.logsPanel.textContent = `No useful log lines remain in the latest window. ${logState.filteredCount} offline polling entries were hidden. Disable the filter to inspect the raw output.`;
        } else {
            ui.logsPanel.textContent = "No log lines are available right now.";
        }
    } else {
        ui.logsPanel.textContent = logState.lines.join("\n");
    }
    ui.logsPanel.scrollTop = ui.logsPanel.scrollHeight;
}

function buildLogSummary() {
    if (!logState.hideOffline) {
        return "Showing raw live logs.";
    }
    if (logState.filteredCount <= 0) {
        return "Offline polling filter is enabled.";
    }
    return `${logState.filteredCount} offline polling log lines are hidden.`;
}

function renderStatus(data) {
    const recorder = data.recorder || {};
    const runtime = data.runtime || {};
    const diagnostics = data.diagnostics || [];

    ui.statusBadge.className = `badge ${recorder.stopping ? "stopping" : recorder.running ? "running" : "stopped"}`;
    ui.statusBadge.textContent = recorder.stopping ? "Stopping" : recorder.running ? "Running" : "Stopped";

    const metrics = [
        ["Enabled Streamers", recorder.enabled_streamers ?? 0],
        ["Scheduled Jobs", recorder.scheduled_jobs ?? 0],
        ["Active Recordings", (recorder.active_recordings || []).length],
        ["Uptime", recorder.uptime || "Not started"]
    ];
    ui.recorderSummary.innerHTML = "";
    metrics.forEach(([label, value]) => {
        const item = document.createElement("div");
        item.className = "metric";
        item.innerHTML = `<span>${label}</span><strong>${value}</strong>`;
        ui.recorderSummary.appendChild(item);
    });

    if (recorder.last_error) {
        const item = document.createElement("div");
        item.className = "metric";
        item.innerHTML = `<span>Latest Error</span><strong>${escapeHtml(recorder.last_error)}</strong>`;
        ui.recorderSummary.appendChild(item);
    }

    ui.activeRecordings.innerHTML = "";
    if (!recorder.active_recordings?.length) {
        ui.activeRecordings.textContent = "No active recordings.";
    } else {
        recorder.active_recordings.forEach((entry) => {
            const item = document.createElement("div");
            item.className = "record-item";
            item.innerHTML = `
                <strong>${escapeHtml(entry.streamer_name || entry.streamer)}</strong>
                <div class="muted-text mono">${escapeHtml(entry.streamer)}</div>
                <div>${escapeHtml(entry.title || "Untitled stream")}</div>
                <div class="muted-text mono">${escapeHtml(entry.filename || "")}</div>
            `;
            ui.activeRecordings.appendChild(item);
        });
    }

    ui.runtimeInfo.innerHTML = "";
    [
        ["Version", runtime.version || "null"],
        ["Git Commit", runtime.git_commit || "-"],
        ["OS / Arch", `${runtime.os || "-"} / ${runtime.arch || "-"}`],
        ["Working Directory", runtime.working_directory || "-"],
        ["Executable", runtime.executable || "-"],
        ["Listen", runtime.listen_address || "-"],
        ["FFmpeg", runtime.ffmpeg_path || "Not found"],
        ["Built-in Auth", runtime.auth_enabled ? "Enabled" : "Disabled"]
    ].forEach(([label, value]) => {
        const dt = document.createElement("dt");
        dt.textContent = label;
        const dd = document.createElement("dd");
        dd.textContent = value;
        ui.runtimeInfo.append(dt, dd);
    });

    ui.diagnostics.innerHTML = "";
    if (!diagnostics.length) {
        ui.diagnostics.textContent = "No diagnostics require attention right now.";
    } else {
        diagnostics.forEach((item) => {
            const box = document.createElement("div");
            box.className = `diagnostic ${item.level || "info"}`;
            box.textContent = item.message;
            ui.diagnostics.appendChild(box);
        });
    }

    ui.startRecorderBtn.disabled = recorder.running || recorder.stopping;
    ui.stopRecorderBtn.disabled = !recorder.running && !recorder.stopping;
}

function renderSettings(settings) {
    ui.langInput.value = settings.app?.lang || "EN";
    ui.enableLogInput.checked = Boolean(settings.app?.enable_log);
    renderStreamers(settings.app?.streamers || []);

    ui.discordEnabledInput.checked = Boolean(settings.discord?.enabled);
    ui.discordTokenInput.value = settings.discord?.bot_token || "";
    ui.discordGuildInput.value = settings.discord?.guild_id || "";
    ui.discordNotifyInput.value = settings.discord?.notify_channel_id || "";
    ui.discordArchiveInput.value = settings.discord?.archive_channel_id || "";
    ui.discordTagRoleInput.checked = Boolean(settings.discord?.tag_role);

    ui.telegramEnabledInput.checked = Boolean(settings.telegram?.enabled);
    ui.telegramTokenInput.value = settings.telegram?.bot_token || "";
    ui.telegramChatInput.value = settings.telegram?.chat_id || "";
    ui.telegramEndpointInput.value = settings.telegram?.api_endpoint || "https://api.telegram.org";
    ui.telegramConvertInput.checked = Boolean(settings.telegram?.convert_to_m4a);
    ui.telegramKeepInput.checked = Boolean(settings.telegram?.keep_original);
}

function renderStreamers(streamers) {
    ui.streamersBody.innerHTML = "";

    if (!streamers.length) {
        const row = document.createElement("tr");
        row.innerHTML = `<td colspan="5" class="muted-text">No streamers are configured yet. Use “Add Streamer” to create one.</td>`;
        ui.streamersBody.appendChild(row);
        return;
    }

    streamers.forEach((streamer) => {
        ui.streamersBody.appendChild(createStreamerRow(streamer));
    });
}

function createStreamerRow(streamer = {}) {
    const row = document.createElement("tr");
    row.dataset.streamerRow = "1";
    row.innerHTML = `
        <td><input type="checkbox" data-field="enabled"></td>
        <td><input type="text" data-field="screen-id" placeholder="mielu_ii"></td>
        <td><input type="text" data-field="schedule" placeholder="@every 5s"></td>
        <td><input type="text" data-field="folder" placeholder="Recordings/streamer-name"></td>
        <td class="actions-cell"><button type="button" class="small danger">Remove</button></td>
    `;

    row.querySelector('[data-field="enabled"]').checked = Boolean(streamer.enabled ?? true);
    row.querySelector('[data-field="screen-id"]').value = streamer["screen-id"] || "";
    row.querySelector('[data-field="schedule"]').value = streamer.schedule || "@every 5s";
    row.querySelector('[data-field="folder"]').value = streamer.folder || "";
    row.querySelector("button").addEventListener("click", () => {
        row.remove();
        if (!ui.streamersBody.querySelector("[data-streamer-row='1']")) {
            renderStreamers([]);
        }
    });

    return row;
}

function addStreamerRow() {
    const placeholder = ui.streamersBody.querySelector("td[colspan='5']");
    if (placeholder) {
        ui.streamersBody.innerHTML = "";
    }
    ui.streamersBody.appendChild(createStreamerRow({ enabled: true, schedule: "@every 5s" }));
}

async function saveSettings() {
    const payload = collectSettings();
    const response = await api("/api/settings", {
        method: "PUT",
        body: JSON.stringify(payload)
    });

    if (response.warning) {
        showToast(response.warning, true);
    } else {
        showToast(response.needs_restart ? "Settings were saved. The recorder is still running, so a restart is recommended before expecting new behavior." : "Settings saved.");
    }
    await refreshAll();
}

// Test actions use the current form values so users can validate credentials before saving them to disk.
async function sendDiscordTest() {
    await runButtonAction(ui.discordTestBtn, "Sending...", async () => {
        const payload = collectDiscordSettings();
        await api("/api/discord/test", {
            method: "POST",
            body: JSON.stringify(payload)
        });
        showToast("Discord test message sent.");
    });
}

async function sendTelegramTest() {
    await runButtonAction(ui.telegramTestBtn, "Sending...", async () => {
        const payload = collectTelegramSettings();
        await api("/api/telegram/test", {
            method: "POST",
            body: JSON.stringify(payload)
        });
        showToast("Telegram test message sent.");
    });
}

async function checkForUpdates() {
    await runButtonAction(ui.checkVersionBtn, "Checking...", async () => {
        const result = await api("/api/version/check");
        if (result.update_available && result.repo_url) {
            showToast("A newer build is available. Redirecting to the repository...");
            window.location.href = result.repo_url;
            return;
        }

        showToast(result.message || "This build already matches origin/main.");
    });
}

async function runButtonAction(button, busyLabel, action) {
    const originalLabel = button.textContent;
    button.disabled = true;
    button.textContent = busyLabel;
    try {
        await action();
    } finally {
        button.disabled = false;
        button.textContent = originalLabel;
    }
}

function collectSettings() {
    const app = collectAppSettings();
    const discord = collectDiscordSettings();
    const telegram = collectTelegramSettings();

    return { app, discord, telegram };
}

function collectAppSettings() {
    const streamers = Array.from(ui.streamersBody.querySelectorAll("[data-streamer-row='1']")).map((row) => ({
        enabled: row.querySelector('[data-field="enabled"]').checked,
        "screen-id": row.querySelector('[data-field="screen-id"]').value.trim(),
        schedule: row.querySelector('[data-field="schedule"]').value.trim(),
        folder: row.querySelector('[data-field="folder"]').value.trim()
    })).filter((streamer) => streamer["screen-id"] || streamer.folder || streamer.schedule);

    return {
        lang: ui.langInput.value,
        enable_log: ui.enableLogInput.checked,
        streamers
    };
}

function collectDiscordSettings() {
    return {
        enabled: ui.discordEnabledInput.checked,
        bot_token: ui.discordTokenInput.value.trim(),
        guild_id: ui.discordGuildInput.value.trim(),
        notify_channel_id: ui.discordNotifyInput.value.trim(),
        archive_channel_id: ui.discordArchiveInput.value.trim(),
        tag_role: ui.discordTagRoleInput.checked
    };
}

function collectTelegramSettings() {
    return {
        enabled: ui.telegramEnabledInput.checked,
        bot_token: ui.telegramTokenInput.value.trim(),
        chat_id: ui.telegramChatInput.value.trim(),
        api_endpoint: ui.telegramEndpointInput.value.trim(),
        convert_to_m4a: ui.telegramConvertInput.checked,
        keep_original: ui.telegramKeepInput.checked
    };
}

async function controlRecorder(action) {
    await api(`/api/recorder/${action}`, { method: "POST" });
    showToast(`Recorder action completed: ${action}.`);
    await Promise.all([loadStatus(), loadLogs()]);
}

async function restartBot() {
    if (!window.confirm("Restart the entire bot process now? The web console and recorder will disconnect briefly.")) {
        return;
    }

    botRestartPending = true;
    ui.restartBotBtn.disabled = true;
    await api("/api/bot/restart", { method: "POST" });
    showToast("Bot restart requested. The page will reconnect automatically in a few seconds.");
    waitForBotRecovery();
}

// After a bot restart the listener disappears briefly, so poll health until the web process comes back.
function waitForBotRecovery() {
    const startedAt = Date.now();

    const poll = async () => {
        try {
            const response = await fetch("/api/status", { cache: "no-store" });
            if (response.ok || response.status === 401) {
                window.location.reload();
                return;
            }
        } catch {}

        if (Date.now() - startedAt >= 60000) {
            botRestartPending = false;
            ui.restartBotBtn.disabled = false;
            showToast("Bot did not recover within 60 seconds. Check web.log or the process output.", true);
            return;
        }

        window.setTimeout(poll, 2000);
    };

    window.setTimeout(poll, 2500);
}

function renderFileRoots(roots) {
    if (!Array.isArray(roots) || !roots.length) {
        ui.fileRootSelect.innerHTML = "";
        return;
    }

    const currentValue = fileState.root || ui.fileRootSelect.value;
    ui.fileRootSelect.innerHTML = "";
    roots.forEach((root) => {
        const option = document.createElement("option");
        option.value = root.root;
        option.textContent = root.exists ? `${root.label} · ${root.root}` : `${root.label} · ${root.root} (not created yet)`;
        ui.fileRootSelect.appendChild(option);
    });

    const matched = roots.find((root) => root.root === currentValue) || roots[0];
    ui.fileRootSelect.value = matched.root;
    if (!fileState.root) {
        fileState.root = matched.root;
    }
}

async function browseFiles(root, path) {
    if (!root) {
        return;
    }

    const data = await api(`/api/files?root=${encodeURIComponent(root)}&path=${encodeURIComponent(path || "")}`);
    fileState.root = data.root;
    fileState.path = data.path || "";
    ui.fileRootSelect.value = data.root;
    ui.filePathLabel.textContent = `/${data.path || ""}`.replace(/\/$/, "") || "/";
    ui.fileUpBtn.disabled = !data.path;

    ui.filesBody.innerHTML = "";
    if (!data.entries?.length) {
        const row = document.createElement("tr");
        row.innerHTML = `<td colspan="5" class="muted-text">This folder is currently empty.</td>`;
        ui.filesBody.appendChild(row);
        return;
    }

    data.entries.forEach((entry) => ui.filesBody.appendChild(createFileRow(entry)));
}

function createFileRow(entry) {
    const row = document.createElement("tr");

    const nameCell = document.createElement("td");
    const typeCell = document.createElement("td");
    const sizeCell = document.createElement("td");
    const modifiedCell = document.createElement("td");
    const actionsCell = document.createElement("td");
    actionsCell.className = "actions-cell";

    if (entry.type === "dir") {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "table-link";
        button.textContent = entry.name;
        button.addEventListener("click", () => browseFiles(fileState.root, entry.path).catch(handleError));
        nameCell.appendChild(button);
    } else {
        const span = document.createElement("span");
        span.textContent = entry.name;
        nameCell.appendChild(span);
    }

    typeCell.textContent = entry.type;
    sizeCell.textContent = entry.type === "dir" ? "—" : formatBytes(entry.size || 0);
    sizeCell.className = "mono";
    modifiedCell.textContent = formatDate(entry.modified_at);

    if (entry.downloadable) {
        const uploadButton = document.createElement("button");
        uploadButton.type = "button";
        uploadButton.className = "table-link";
        uploadButton.textContent = actionsCell.childNodes.length ? " / Upload" : "Upload";
        uploadButton.addEventListener("click", async () => {
            if (!window.confirm(`Upload ${entry.name} to Telegram now?`)) {
                return;
            }

            const originalLabel = uploadButton.textContent;
            uploadButton.disabled = true;
            uploadButton.textContent = actionsCell.childNodes.length ? " / Uploading..." : "Uploading...";
            try {
                const result = await api("/api/files/telegram-upload", {
                    method: "POST",
                    body: JSON.stringify({ root: fileState.root, path: entry.path })
                });
                const modeLabel = result.method === "audio" ? "audio" : "document";
                showToast(`${entry.name} was uploaded to Telegram as ${modeLabel}.`);
            } finally {
                uploadButton.disabled = false;
                uploadButton.textContent = originalLabel;
            }
        });
        actionsCell.appendChild(uploadButton);

        const downloadLink = document.createElement("a");
        downloadLink.className = "table-link";
        downloadLink.textContent = actionsCell.childNodes.length ? " / Download" : "Download";
        downloadLink.href = `/api/files/download?root=${encodeURIComponent(fileState.root)}&path=${encodeURIComponent(entry.path)}`;
        downloadLink.setAttribute("download", "");
        actionsCell.appendChild(downloadLink);
    }

    if (entry.deletable) {
        const deleteButton = document.createElement("button");
        deleteButton.type = "button";
        deleteButton.className = "table-link";
        deleteButton.textContent = actionsCell.childNodes.length ? " / Delete" : "Delete";
        deleteButton.addEventListener("click", async () => {
            if (!window.confirm(`Delete ${entry.name}?`)) {
                return;
            }
            await api("/api/files/delete", {
                method: "POST",
                body: JSON.stringify({ root: fileState.root, path: entry.path })
            });
            showToast(`${entry.name} was deleted.`);
            await browseFiles(fileState.root, fileState.path);
        });
        actionsCell.appendChild(deleteButton);
    }

    row.append(nameCell, typeCell, sizeCell, modifiedCell, actionsCell);
    return row;
}

function parentPath(path) {
    if (!path) {
        return "";
    }
    const parts = path.split("/").filter(Boolean);
    parts.pop();
    return parts.join("/");
}

function formatBytes(value) {
    if (!value) {
        return "0 B";
    }
    const units = ["B", "KB", "MB", "GB", "TB"];
    let size = value;
    let index = 0;
    while (size >= 1024 && index < units.length - 1) {
        size /= 1024;
        index += 1;
    }
    return `${size.toFixed(size >= 10 || index === 0 ? 0 : 1)} ${units[index]}`;
}

function formatDate(value) {
    if (!value) {
        return "—";
    }
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) {
        return value;
    }
    return parsed.toLocaleString();
}

function showToast(message, isError = false) {
    ui.toast.textContent = message;
    ui.toast.className = `toast show${isError ? " error" : ""}`;
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(() => {
        ui.toast.className = "toast";
    }, 3600);
}

function handleError(error) {
    console.error(error);
    if (botRestartPending) {
        return;
    }
    showToast(error.message || "An unexpected error occurred.", true);
}

function escapeHtml(value) {
    return String(value)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#39;");
}
