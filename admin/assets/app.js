const ui = {};
const fileState = { root: "", path: "" };
let toastTimer = null;

window.addEventListener("DOMContentLoaded", () => {
    cacheElements();
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

    ui.telegramEnabledInput = document.getElementById("telegramEnabledInput");
    ui.telegramTokenInput = document.getElementById("telegramTokenInput");
    ui.telegramChatInput = document.getElementById("telegramChatInput");
    ui.telegramEndpointInput = document.getElementById("telegramEndpointInput");
    ui.telegramConvertInput = document.getElementById("telegramConvertInput");
    ui.telegramKeepInput = document.getElementById("telegramKeepInput");

    ui.fileRootSelect = document.getElementById("fileRootSelect");
    ui.filePathLabel = document.getElementById("filePathLabel");
    ui.fileUpBtn = document.getElementById("fileUpBtn");
    ui.filesBody = document.getElementById("filesBody");
    ui.filePreview = document.getElementById("filePreview");
    ui.logsPanel = document.getElementById("logsPanel");

    ui.refreshBtn = document.getElementById("refreshBtn");
    ui.saveSettingsBtn = document.getElementById("saveSettingsBtn");
    ui.startRecorderBtn = document.getElementById("startRecorderBtn");
    ui.stopRecorderBtn = document.getElementById("stopRecorderBtn");
    ui.restartRecorderBtn = document.getElementById("restartRecorderBtn");
    ui.addStreamerBtn = document.getElementById("addStreamerBtn");
    ui.fileRefreshBtn = document.getElementById("fileRefreshBtn");
    ui.logsRefreshBtn = document.getElementById("logsRefreshBtn");
    ui.clearPreviewBtn = document.getElementById("clearPreviewBtn");
    ui.toast = document.getElementById("toast");
}

function bindEvents() {
    ui.refreshBtn.addEventListener("click", () => refreshAll().catch(handleError));
    ui.saveSettingsBtn.addEventListener("click", () => saveSettings().catch(handleError));
    ui.startRecorderBtn.addEventListener("click", () => controlRecorder("start").catch(handleError));
    ui.stopRecorderBtn.addEventListener("click", () => controlRecorder("stop").catch(handleError));
    ui.restartRecorderBtn.addEventListener("click", () => controlRecorder("restart").catch(handleError));
    ui.addStreamerBtn.addEventListener("click", addStreamerRow);
    ui.fileRootSelect.addEventListener("change", () => browseFiles(ui.fileRootSelect.value, "").catch(handleError));
    ui.fileUpBtn.addEventListener("click", () => browseFiles(fileState.root, parentPath(fileState.path)).catch(handleError));
    ui.fileRefreshBtn.addEventListener("click", () => browseFiles(fileState.root, fileState.path).catch(handleError));
    ui.logsRefreshBtn.addEventListener("click", () => loadLogs().catch(handleError));
    ui.clearPreviewBtn.addEventListener("click", () => {
        ui.filePreview.textContent = "尚未選擇檔案。";
    });
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
    const data = await api("/api/logs?limit=250");
    const lines = data.lines || [];
    ui.logsPanel.textContent = lines.length ? lines.join("\n") : "目前沒有可顯示的日誌。";
    ui.logsPanel.scrollTop = ui.logsPanel.scrollHeight;
}

function renderStatus(data) {
    const recorder = data.recorder || {};
    const runtime = data.runtime || {};
    const diagnostics = data.diagnostics || [];

    ui.statusBadge.className = `badge ${recorder.stopping ? "stopping" : recorder.running ? "running" : "stopped"}`;
    ui.statusBadge.textContent = recorder.stopping ? "Stopping" : recorder.running ? "Running" : "Stopped";

    const metrics = [
        ["已啟用直播主", recorder.enabled_streamers ?? 0],
        ["總排程數", recorder.scheduled_jobs ?? 0],
        ["活動錄影數", (recorder.active_recordings || []).length],
        ["運行時間", recorder.uptime || "未啟動"]
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
        item.innerHTML = `<span>最近錯誤</span><strong>${escapeHtml(recorder.last_error)}</strong>`;
        ui.recorderSummary.appendChild(item);
    }

    ui.activeRecordings.innerHTML = "";
    if (!recorder.active_recordings?.length) {
        ui.activeRecordings.textContent = "目前沒有活動中的錄影。";
    } else {
        recorder.active_recordings.forEach((entry) => {
            const item = document.createElement("div");
            item.className = "record-item";
            item.innerHTML = `
                <strong>${escapeHtml(entry.streamer_name || entry.streamer)}</strong>
                <div class="muted-text mono">${escapeHtml(entry.streamer)}</div>
                <div>${escapeHtml(entry.title || "未命名直播")}</div>
                <div class="muted-text mono">${escapeHtml(entry.filename || "")}</div>
            `;
            ui.activeRecordings.appendChild(item);
        });
    }

    ui.runtimeInfo.innerHTML = "";
    [
        ["OS / Arch", `${runtime.os || "-"} / ${runtime.arch || "-"}`],
        ["Working Directory", runtime.working_directory || "-"],
        ["Executable", runtime.executable || "-"],
        ["Listen", runtime.listen_address || "-"],
        ["FFmpeg", runtime.ffmpeg_path || "未找到"],
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
        ui.diagnostics.textContent = "目前沒有偵測到需要特別處理的項目。";
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
    ui.langInput.value = settings.app?.lang || "ZH";
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
        row.innerHTML = `<td colspan="5" class="muted-text">尚未設定任何直播主。按右上角「新增直播主」即可建立。</td>`;
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
        <td class="actions-cell"><button type="button" class="small danger">移除</button></td>
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
        showToast(response.needs_restart ? "設定已儲存，錄影器目前正在運行，建議重新啟動以套用新設定。" : "設定已儲存。");
    }
    await refreshAll();
}

function collectSettings() {
    const streamers = Array.from(ui.streamersBody.querySelectorAll("[data-streamer-row='1']")).map((row) => ({
        enabled: row.querySelector('[data-field="enabled"]').checked,
        "screen-id": row.querySelector('[data-field="screen-id"]').value.trim(),
        schedule: row.querySelector('[data-field="schedule"]').value.trim(),
        folder: row.querySelector('[data-field="folder"]').value.trim()
    })).filter((streamer) => streamer["screen-id"] || streamer.folder || streamer.schedule);

    return {
        app: {
            lang: ui.langInput.value,
            enable_log: ui.enableLogInput.checked,
            streamers
        },
        discord: {
            enabled: ui.discordEnabledInput.checked,
            bot_token: ui.discordTokenInput.value.trim(),
            guild_id: ui.discordGuildInput.value.trim(),
            notify_channel_id: ui.discordNotifyInput.value.trim(),
            archive_channel_id: ui.discordArchiveInput.value.trim(),
            tag_role: ui.discordTagRoleInput.checked
        },
        telegram: {
            enabled: ui.telegramEnabledInput.checked,
            bot_token: ui.telegramTokenInput.value.trim(),
            chat_id: ui.telegramChatInput.value.trim(),
            api_endpoint: ui.telegramEndpointInput.value.trim(),
            convert_to_m4a: ui.telegramConvertInput.checked,
            keep_original: ui.telegramKeepInput.checked
        }
    };
}

async function controlRecorder(action) {
    await api(`/api/recorder/${action}`, { method: "POST" });
    showToast(`錄影器已執行 ${action}。`);
    await Promise.all([loadStatus(), loadLogs()]);
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
        option.textContent = root.exists ? `${root.label} · ${root.root}` : `${root.label} · ${root.root} (尚未建立)`;
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
        row.innerHTML = `<td colspan="5" class="muted-text">這個資料夾目前是空的。</td>`;
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

    if (entry.previewable) {
        const previewButton = document.createElement("button");
        previewButton.type = "button";
        previewButton.className = "table-link";
        previewButton.textContent = "預覽";
        previewButton.addEventListener("click", () => previewFile(entry.path).catch(handleError));
        actionsCell.appendChild(previewButton);
    }

    if (entry.downloadable) {
        const downloadLink = document.createElement("a");
        downloadLink.className = "table-link";
        downloadLink.textContent = entry.previewable ? " / 下載" : "下載";
        downloadLink.href = `/api/files/download?root=${encodeURIComponent(fileState.root)}&path=${encodeURIComponent(entry.path)}`;
        downloadLink.setAttribute("download", "");
        actionsCell.appendChild(downloadLink);
    }

    if (entry.deletable) {
        const deleteButton = document.createElement("button");
        deleteButton.type = "button";
        deleteButton.className = "table-link";
        deleteButton.textContent = actionsCell.childNodes.length ? " / 刪除" : "刪除";
        deleteButton.addEventListener("click", async () => {
            if (!window.confirm(`確定要刪除 ${entry.name} 嗎？`)) {
                return;
            }
            await api("/api/files/delete", {
                method: "POST",
                body: JSON.stringify({ root: fileState.root, path: entry.path })
            });
            showToast(`已刪除 ${entry.name}`);
            await browseFiles(fileState.root, fileState.path);
        });
        actionsCell.appendChild(deleteButton);
    }

    row.append(nameCell, typeCell, sizeCell, modifiedCell, actionsCell);
    return row;
}

async function previewFile(path) {
    const data = await api(`/api/files/content?root=${encodeURIComponent(fileState.root)}&path=${encodeURIComponent(path)}`);
    ui.filePreview.textContent = data.truncated ? `${data.content}\n\n[預覽已截斷]` : data.content;
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
    showToast(error.message || "發生未預期錯誤。", true);
}

function escapeHtml(value) {
    return String(value)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#39;");
}
