const ui = {};
const fileState = { root: "", path: "" };
const THEME_STORAGE_KEY = "twitcast-theme";
const HIDE_OFFLINE_LOGS_STORAGE_KEY = "twitcast-hide-offline-logs";
const LANG_STORAGE_KEY = "twitcast-ui-lang";
const langState = { value: "EN" };
const themeState = { value: "dark" };
const logState = { lines: [], hideOffline: true, filteredCount: 0, loaded: false };
const appState = { status: null, files: null };
let botRestartPending = false;
let toastTimer = null;

const I18N = {
    EN: {
        "document.title": "TwitCasting VPS Console",
        "hero.eyebrow": "Headless Linux Control Room",
        "hero.title": "TwitCasting Recorder Console",
        "hero.lede": "Manage recording, notifications, files, and update checks from a single web console designed for Ubuntu VPS deployments.",
        "actions.refresh": "Refresh",
        "actions.save": "Save Settings",
        "actions.startRecorder": "Start Recorder",
        "actions.stopRecorder": "Stop Recorder",
        "actions.restartRecorder": "Restart Recorder",
        "actions.restartBot": "Restart Bot",
        "theme.useLight": "Use Light Theme",
        "theme.useDark": "Use Dark Theme",
        "theme.ariaLight": "Switch to light theme",
        "theme.ariaDark": "Switch to dark theme",
        "status.section": "Service Status",
        "status.activeRecordings": "Active Recordings",
        "status.noActiveRecordings": "No active recordings.",
        "status.running": "Running",
        "status.stopped": "Stopped",
        "status.stopping": "Stopping",
        "status.untitledStream": "Untitled stream",
        "system.section": "System Info",
        "system.checkUpdates": "Check for Updates",
        "system.diagnostics": "Diagnostics",
        "system.diagnosticsPending": "Diagnostics have not been loaded yet.",
        "system.version": "Version",
        "system.gitCommit": "Git Commit",
        "system.osArch": "OS / Arch",
        "system.workingDirectory": "Working Directory",
        "system.executable": "Executable",
        "system.listen": "Listen",
        "system.ffmpeg": "FFmpeg",
        "system.builtInAuth": "Built-in Auth",
        "system.notFound": "Not found",
        "system.noDiagnostics": "No diagnostics require attention right now.",
        "system.checking": "Checking...",
        "system.redirecting": "A newer build is available. Redirecting to the repository...",
        "system.updateAvailable": "A newer build is available.",
        "system.openRepository": "Open repository",
        "system.upToDate": "This build already matches origin/main.",
        "settings.section": "General & Streamer Settings",
        "settings.addStreamer": "Add Streamer",
        "settings.recorderLanguage": "Recorder Language",
        "settings.langEnglish": "English",
        "settings.langChinese": "Traditional Chinese",
        "settings.mirrorLogs": "Mirror logs to `app.log`",
        "settings.passwordHint": "If the streamer uses a TwitCasting secret word, enter it in the password column. Leave it blank for normal streams.",
        "table.enabled": "Enabled",
        "table.screenId": "Screen ID",
        "table.schedule": "Schedule",
        "table.streamPassword": "Live Password",
        "table.targetFolder": "Target Folder",
        "table.name": "Name",
        "table.type": "Type",
        "table.size": "Size",
        "table.modified": "Modified",
        "discord.section": "Discord Notifications",
        "discord.testButton": "Send Test Message",
        "discord.enabled": "Enable Discord notifications",
        "discord.botToken": "Bot Token",
        "discord.guildId": "Guild ID",
        "discord.notifyChannel": "Notify Channel ID",
        "discord.archiveChannel": "Archive Channel ID",
        "discord.tagRole": "Mention the per-streamer role when a stream starts",
        "discord.sending": "Sending...",
        "discord.sent": "Discord test message sent.",
        "telegram.section": "Telegram & Conversion",
        "telegram.testButton": "Send Test Message",
        "telegram.enabled": "Enable Telegram upload",
        "telegram.botToken": "Bot Token",
        "telegram.chatId": "Chat ID",
        "telegram.apiEndpoint": "API Endpoint",
        "telegram.convertToM4A": "Extract M4A audio after each recording",
        "telegram.keepOriginal": "Keep the original TS file after conversion",
        "telegram.sending": "Sending...",
        "telegram.sent": "Telegram test message sent.",
        "files.section": "File Manager",
        "files.upOneLevel": "Up One Level",
        "files.root": "Root",
        "files.notCreatedYet": "not created yet",
        "files.empty": "This folder is currently empty.",
        "files.upload": "Upload",
        "files.uploading": "Uploading...",
        "files.uploadConfirm": "Upload {name} to Telegram now?",
        "files.uploaded": "{name} was uploaded to Telegram as {mode}.",
        "files.download": "Download",
        "files.delete": "Delete",
        "files.deleteConfirm": "Delete {name}?",
        "files.deleted": "{name} was deleted.",
        "files.methodAudio": "audio",
        "files.methodDocument": "document",
        "logs.section": "Live Logs",
        "logs.hideOffline": "Hide offline polling",
        "logs.noUseful": "No useful log lines remain in the latest window. {count} offline polling entries were hidden. Disable the filter to inspect the raw output.",
        "logs.noLines": "No log lines are available right now.",
        "logs.raw": "Showing raw live logs.",
        "logs.filterEnabled": "Offline polling filter is enabled.",
        "logs.filteredCount": "{count} offline polling log lines are hidden.",
        "metrics.enabledStreamers": "Enabled Streamers",
        "metrics.scheduledJobs": "Scheduled Jobs",
        "metrics.activeRecordings": "Active Recordings",
        "metrics.uptime": "Uptime",
        "metrics.latestError": "Latest Error",
        "metrics.notStarted": "Not started",
        "streamers.none": "No streamers are configured yet. Use \"Add Streamer\" to create one.",
        "streamers.placeholderPassword": "Optional",
        "streamers.placeholderFolder": "Recordings/streamer-name",
        "streamers.remove": "Remove",
        "save.saved": "Settings saved.",
        "save.savedRestartRecommended": "Settings were saved. The recorder is still running, so a restart is recommended before expecting new behavior.",
        "recorder.start": "start",
        "recorder.stop": "stop",
        "recorder.restart": "restart",
        "recorder.actionCompleted": "Recorder action completed: {action}.",
        "bot.restartConfirm": "Restart the entire bot process now? The web console and recorder will disconnect briefly.",
        "bot.restartRequested": "Bot restart requested. The page will reconnect automatically in a few seconds.",
        "bot.notRecovered": "Bot did not recover within 60 seconds. Check web.log or the process output.",
        "common.loading": "Loading...",
        "common.enabled": "Enabled",
        "common.disabled": "Disabled",
        "common.requestFailed": "Request failed",
        "common.unexpectedError": "An unexpected error occurred."
    },
    ZH: {
        "document.title": "TwitCasting VPS 控制台",
        "hero.eyebrow": "無頭 Linux 控制室",
        "hero.title": "TwitCasting 錄影管理台",
        "hero.lede": "在為 Ubuntu VPS 設計的單一管理頁中，集中處理錄影、通知、檔案與更新檢查。",
        "actions.refresh": "重新整理",
        "actions.save": "儲存設定",
        "actions.startRecorder": "啟動錄影器",
        "actions.stopRecorder": "停止錄影器",
        "actions.restartRecorder": "重啟錄影器",
        "actions.restartBot": "重啟 Bot",
        "theme.useLight": "切換為亮色主題",
        "theme.useDark": "切換為暗色主題",
        "theme.ariaLight": "切換為亮色主題",
        "theme.ariaDark": "切換為暗色主題",
        "status.section": "服務狀態",
        "status.activeRecordings": "進行中的錄影",
        "status.noActiveRecordings": "目前沒有進行中的錄影。",
        "status.running": "執行中",
        "status.stopped": "已停止",
        "status.stopping": "停止中",
        "status.untitledStream": "未命名直播",
        "system.section": "系統資訊",
        "system.checkUpdates": "檢查更新",
        "system.diagnostics": "診斷資訊",
        "system.diagnosticsPending": "診斷資訊尚未載入。",
        "system.version": "版本",
        "system.gitCommit": "Git Commit",
        "system.osArch": "作業系統 / 架構",
        "system.workingDirectory": "工作目錄",
        "system.executable": "執行檔",
        "system.listen": "監聽位址",
        "system.ffmpeg": "FFmpeg",
        "system.builtInAuth": "內建驗證",
        "system.notFound": "未找到",
        "system.noDiagnostics": "目前沒有需要特別注意的診斷項目。",
        "system.checking": "檢查中...",
        "system.redirecting": "發現較新的版本，正在跳轉到倉庫頁面...",
        "system.updateAvailable": "發現較新的版本。",
        "system.openRepository": "前往倉庫",
        "system.upToDate": "目前版本已與 origin/main 一致。",
        "settings.section": "一般與直播主設定",
        "settings.addStreamer": "新增直播主",
        "settings.recorderLanguage": "錄影器語言",
        "settings.langEnglish": "英文",
        "settings.langChinese": "繁體中文",
        "settings.mirrorLogs": "同步寫入 `app.log`",
        "settings.passwordHint": "若該直播主使用 TwitCasting 合言葉，請在密碼欄填入；一般直播則留白即可。",
        "table.enabled": "啟用",
        "table.screenId": "Screen ID",
        "table.schedule": "排程",
        "table.streamPassword": "直播密碼",
        "table.targetFolder": "目標資料夾",
        "table.name": "名稱",
        "table.type": "類型",
        "table.size": "大小",
        "table.modified": "修改時間",
        "discord.section": "Discord 通知",
        "discord.testButton": "發送測試訊息",
        "discord.enabled": "啟用 Discord 通知",
        "discord.botToken": "Bot Token",
        "discord.guildId": "Guild ID",
        "discord.notifyChannel": "通知頻道 ID",
        "discord.archiveChannel": "歸檔頻道 ID",
        "discord.tagRole": "直播開始時提及該直播主的專屬身分組",
        "discord.sending": "發送中...",
        "discord.sent": "Discord 測試訊息已送出。",
        "telegram.section": "Telegram 與轉檔",
        "telegram.testButton": "發送測試訊息",
        "telegram.enabled": "啟用 Telegram 上傳",
        "telegram.botToken": "Bot Token",
        "telegram.chatId": "Chat ID",
        "telegram.apiEndpoint": "API Endpoint",
        "telegram.convertToM4A": "每次錄影後抽取 M4A 音訊",
        "telegram.keepOriginal": "轉檔後保留原始 TS 檔",
        "telegram.sending": "發送中...",
        "telegram.sent": "Telegram 測試訊息已送出。",
        "files.section": "檔案管理",
        "files.upOneLevel": "回上一層",
        "files.root": "根目錄",
        "files.notCreatedYet": "尚未建立",
        "files.empty": "這個資料夾目前是空的。",
        "files.upload": "上傳",
        "files.uploading": "上傳中...",
        "files.uploadConfirm": "現在要把 {name} 上傳到 Telegram 嗎？",
        "files.uploaded": "{name} 已作為 {mode} 上傳到 Telegram。",
        "files.download": "下載",
        "files.delete": "刪除",
        "files.deleteConfirm": "確定要刪除 {name} 嗎？",
        "files.deleted": "{name} 已刪除。",
        "files.methodAudio": "音訊",
        "files.methodDocument": "文件",
        "logs.section": "即時日誌",
        "logs.hideOffline": "隱藏離線輪詢",
        "logs.noUseful": "最新視窗內沒有可用的日誌行。已隱藏 {count} 筆離線輪詢訊息，如需查看原始輸出請關閉過濾。",
        "logs.noLines": "目前沒有可顯示的日誌行。",
        "logs.raw": "目前顯示原始即時日誌。",
        "logs.filterEnabled": "已啟用離線輪詢過濾。",
        "logs.filteredCount": "已隱藏 {count} 筆離線輪詢日誌。",
        "metrics.enabledStreamers": "已啟用直播主",
        "metrics.scheduledJobs": "排程數量",
        "metrics.activeRecordings": "錄影中數量",
        "metrics.uptime": "運行時間",
        "metrics.latestError": "最近錯誤",
        "metrics.notStarted": "尚未啟動",
        "streamers.none": "目前尚未設定任何直播主，請使用「新增直播主」建立一筆。",
        "streamers.placeholderPassword": "選填",
        "streamers.placeholderFolder": "錄影/直播主名稱",
        "streamers.remove": "移除",
        "save.saved": "設定已儲存。",
        "save.savedRestartRecommended": "設定已儲存，但錄影器仍在執行中；若要套用新行為，建議先重啟錄影器。",
        "recorder.start": "啟動",
        "recorder.stop": "停止",
        "recorder.restart": "重啟",
        "recorder.actionCompleted": "錄影器操作已完成：{action}。",
        "bot.restartConfirm": "現在要重啟整個 Bot 行程嗎？管理頁與錄影器會暫時中斷。",
        "bot.restartRequested": "已送出 Bot 重啟要求，頁面會在幾秒內自動重新連線。",
        "bot.notRecovered": "Bot 在 60 秒內尚未恢復，請檢查 web.log 或程序輸出。",
        "common.loading": "載入中...",
        "common.enabled": "啟用",
        "common.disabled": "停用",
        "common.requestFailed": "請求失敗",
        "common.unexpectedError": "發生未預期的錯誤。"
    }
};

function normalizeLanguage(language) {
    return language === "ZH" ? "ZH" : "EN";
}

function readStoredLanguage() {
    try {
        return normalizeLanguage(window.localStorage.getItem(LANG_STORAGE_KEY));
    } catch {
        return "EN";
    }
}

function t(key, params = {}) {
    const language = I18N[langState.value] ? langState.value : "EN";
    const template = I18N[language][key] ?? I18N.EN[key] ?? key;
    return template.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? ""));
}

window.addEventListener("DOMContentLoaded", () => {
    cacheElements();
    applyLanguage(readStoredLanguage());
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

// 語言切換需要同時更新靜態標籤與目前畫面上的動態內容，避免只存設定但畫面不變。
function applyLanguage(language) {
    langState.value = normalizeLanguage(language);
    document.documentElement.lang = langState.value === "ZH" ? "zh-Hant" : "en";
    document.title = t("document.title");

    try {
        window.localStorage.setItem(LANG_STORAGE_KEY, langState.value);
    } catch {}

    document.querySelectorAll("[data-i18n]").forEach((element) => {
        element.textContent = t(element.dataset.i18n);
    });

    applyTheme(themeState.value);

    if (appState.status) {
        renderStatus(appState.status);
    } else {
        ui.statusBadge.textContent = t("common.loading");
        ui.activeRecordings.textContent = t("status.noActiveRecordings");
        ui.diagnostics.textContent = t("system.diagnosticsPending");
    }

    if (appState.files) {
        renderFiles(appState.files);
    }

    if (ui.streamersBody?.querySelector("[data-streamer-row='1'], td[colspan]")) {
        renderStreamers(collectStreamerRows());
    }

    renderLogs();
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
    ui.langInput.addEventListener("change", () => applyLanguage(ui.langInput.value));
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
        ui.themeToggleBtn.textContent = isDark ? t("theme.useLight") : t("theme.useDark");
        ui.themeToggleBtn.setAttribute("aria-label", isDark ? t("theme.ariaLight") : t("theme.ariaDark"));
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
        throw new Error(payload?.error || response.statusText || t("common.requestFailed"));
    }
    return payload;
}

async function loadStatus() {
    const data = await api("/api/status");
    appState.status = data;
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
    logState.loaded = true;
    renderLogs();
}

// Filter offline polling on the backend first so noisy lines do not push useful logs out of the latest window.
function renderLogs() {
    if (!logState.loaded) {
        ui.logsSummary.textContent = "";
        ui.logsPanel.textContent = t("common.loading");
        return;
    }

    ui.logsSummary.textContent = buildLogSummary();

    if (!logState.lines.length) {
        if (logState.hideOffline && logState.filteredCount > 0) {
            ui.logsPanel.textContent = t("logs.noUseful", { count: logState.filteredCount });
        } else {
            ui.logsPanel.textContent = t("logs.noLines");
        }
    } else {
        ui.logsPanel.textContent = logState.lines.join("\n");
    }
    ui.logsPanel.scrollTop = ui.logsPanel.scrollHeight;
}

function buildLogSummary() {
    if (!logState.hideOffline) {
        return t("logs.raw");
    }
    if (logState.filteredCount <= 0) {
        return t("logs.filterEnabled");
    }
    return t("logs.filteredCount", { count: logState.filteredCount });
}

function renderStatus(data) {
    const recorder = data.recorder || {};
    const runtime = data.runtime || {};
    const diagnostics = data.diagnostics || [];

    ui.statusBadge.className = `badge ${recorder.stopping ? "stopping" : recorder.running ? "running" : "stopped"}`;
    ui.statusBadge.textContent = recorder.stopping ? t("status.stopping") : recorder.running ? t("status.running") : t("status.stopped");

    const metrics = [
        [t("metrics.enabledStreamers"), recorder.enabled_streamers ?? 0],
        [t("metrics.scheduledJobs"), recorder.scheduled_jobs ?? 0],
        [t("metrics.activeRecordings"), (recorder.active_recordings || []).length],
        [t("metrics.uptime"), recorder.uptime || t("metrics.notStarted")]
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
        item.innerHTML = `<span>${t("metrics.latestError")}</span><strong>${escapeHtml(recorder.last_error)}</strong>`;
        ui.recorderSummary.appendChild(item);
    }

    ui.activeRecordings.innerHTML = "";
    if (!recorder.active_recordings?.length) {
        ui.activeRecordings.textContent = t("status.noActiveRecordings");
    } else {
        recorder.active_recordings.forEach((entry) => {
            const item = document.createElement("div");
            item.className = "record-item";
            item.innerHTML = `
                <strong>${escapeHtml(entry.streamer_name || entry.streamer)}</strong>
                <div class="muted-text mono">${escapeHtml(entry.streamer)}</div>
                <div>${escapeHtml(entry.title || t("status.untitledStream"))}</div>
                <div class="muted-text mono">${escapeHtml(entry.filename || "")}</div>
            `;
            ui.activeRecordings.appendChild(item);
        });
    }

    ui.runtimeInfo.innerHTML = "";
    [
        [t("system.version"), runtime.version || "null"],
        [t("system.gitCommit"), runtime.git_commit || "-"],
        [t("system.osArch"), `${runtime.os || "-"} / ${runtime.arch || "-"}`],
        [t("system.workingDirectory"), runtime.working_directory || "-"],
        [t("system.executable"), runtime.executable || "-"],
        [t("system.listen"), runtime.listen_address || "-"],
        [t("system.ffmpeg"), runtime.ffmpeg_path || t("system.notFound")],
        [t("system.builtInAuth"), runtime.auth_enabled ? t("common.enabled") : t("common.disabled")]
    ].forEach(([label, value]) => {
        const dt = document.createElement("dt");
        dt.textContent = label;
        const dd = document.createElement("dd");
        dd.textContent = value;
        ui.runtimeInfo.append(dt, dd);
    });

    ui.diagnostics.innerHTML = "";
    if (!diagnostics.length) {
        ui.diagnostics.textContent = t("system.noDiagnostics");
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
    applyLanguage(ui.langInput.value);
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
        row.innerHTML = `<td colspan="6" class="muted-text">${escapeHtml(t("streamers.none"))}</td>`;
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
        <td><input type="password" data-field="password" autocomplete="new-password" placeholder="${escapeHtml(t("streamers.placeholderPassword"))}"></td>
        <td><input type="text" data-field="folder" placeholder="${escapeHtml(t("streamers.placeholderFolder"))}"></td>
        <td class="actions-cell"><button type="button" class="small danger">${escapeHtml(t("streamers.remove"))}</button></td>
    `;

    row.querySelector('[data-field="enabled"]').checked = Boolean(streamer.enabled ?? true);
    row.querySelector('[data-field="screen-id"]').value = streamer["screen-id"] || "";
    row.querySelector('[data-field="schedule"]').value = streamer.schedule || "@every 5s";
    row.querySelector('[data-field="password"]').value = streamer.password || "";
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
    const placeholder = ui.streamersBody.querySelector("td[colspan='6']");
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
        showToast(response.needs_restart ? t("save.savedRestartRecommended") : t("save.saved"));
    }
    await refreshAll();
}

// Test actions use the current form values so users can validate credentials before saving them to disk.
async function sendDiscordTest() {
    await runButtonAction(ui.discordTestBtn, t("discord.sending"), async () => {
        const payload = collectDiscordSettings();
        await api("/api/discord/test", {
            method: "POST",
            body: JSON.stringify(payload)
        });
        showToast(t("discord.sent"));
    });
}

async function sendTelegramTest() {
    await runButtonAction(ui.telegramTestBtn, t("telegram.sending"), async () => {
        const payload = collectTelegramSettings();
        await api("/api/telegram/test", {
            method: "POST",
            body: JSON.stringify(payload)
        });
        showToast(t("telegram.sent"));
    });
}

async function checkForUpdates() {
    await runButtonAction(ui.checkVersionBtn, t("system.checking"), async () => {
        const result = await api("/api/version/check");
        if (result.update_available && result.repo_url) {
            showToastLink(t("system.updateAvailable"), t("system.openRepository"), result.repo_url);
            return;
        }

        if (!result.update_available && (result.current_commit || result.latest_commit)) {
            showToast(t("system.upToDate"));
            return;
        }

        showToast(result.message || t("system.upToDate"));
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
    const streamers = collectStreamerRows().filter((streamer) => streamer["screen-id"] || streamer.folder || streamer.schedule || streamer.password);

    return {
        lang: ui.langInput.value,
        enable_log: ui.enableLogInput.checked,
        streamers
    };
}

function collectStreamerRows() {
    return Array.from(ui.streamersBody.querySelectorAll("[data-streamer-row='1']")).map((row) => ({
        enabled: row.querySelector('[data-field="enabled"]').checked,
        "screen-id": row.querySelector('[data-field="screen-id"]').value.trim(),
        schedule: row.querySelector('[data-field="schedule"]').value.trim(),
        password: row.querySelector('[data-field="password"]').value.trim(),
        folder: row.querySelector('[data-field="folder"]').value.trim()
    }));
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
    showToast(t("recorder.actionCompleted", { action: t(`recorder.${action}`) }));
    await Promise.all([loadStatus(), loadLogs()]);
}

async function restartBot() {
    if (!window.confirm(t("bot.restartConfirm"))) {
        return;
    }

    botRestartPending = true;
    ui.restartBotBtn.disabled = true;
    await api("/api/bot/restart", { method: "POST" });
    showToast(t("bot.restartRequested"));
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
            showToast(t("bot.notRecovered"), true);
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
        option.textContent = root.exists ? `${root.label} · ${root.root}` : `${root.label} · ${root.root} (${t("files.notCreatedYet")})`;
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
    appState.files = data;
    renderFiles(data);
}

function renderFiles(data) {
    fileState.root = data.root;
    fileState.path = data.path || "";
    ui.fileRootSelect.value = data.root;
    ui.filePathLabel.textContent = `/${data.path || ""}`.replace(/\/$/, "") || "/";
    ui.fileUpBtn.disabled = !data.path;

    ui.filesBody.innerHTML = "";
    if (!data.entries?.length) {
        const row = document.createElement("tr");
        row.innerHTML = `<td colspan="5" class="muted-text">${escapeHtml(t("files.empty"))}</td>`;
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
        uploadButton.textContent = prefixedActionLabel(actionsCell.childNodes.length > 0, t("files.upload"));
        uploadButton.addEventListener("click", async () => {
            if (!window.confirm(t("files.uploadConfirm", { name: entry.name }))) {
                return;
            }

            const originalLabel = uploadButton.textContent;
            uploadButton.disabled = true;
            uploadButton.textContent = prefixedActionLabel(actionsCell.childNodes.length > 0, t("files.uploading"));
            try {
                const result = await api("/api/files/telegram-upload", {
                    method: "POST",
                    body: JSON.stringify({ root: fileState.root, path: entry.path })
                });
                const modeLabel = result.method === "audio" ? t("files.methodAudio") : t("files.methodDocument");
                showToast(t("files.uploaded", { name: entry.name, mode: modeLabel }));
            } finally {
                uploadButton.disabled = false;
                uploadButton.textContent = originalLabel;
            }
        });
        actionsCell.appendChild(uploadButton);

        const downloadLink = document.createElement("a");
        downloadLink.className = "table-link";
        downloadLink.textContent = prefixedActionLabel(actionsCell.childNodes.length > 0, t("files.download"));
        downloadLink.href = `/api/files/download?root=${encodeURIComponent(fileState.root)}&path=${encodeURIComponent(entry.path)}`;
        downloadLink.setAttribute("download", "");
        actionsCell.appendChild(downloadLink);
    }

    if (entry.deletable) {
        const deleteButton = document.createElement("button");
        deleteButton.type = "button";
        deleteButton.className = "table-link";
        deleteButton.textContent = prefixedActionLabel(actionsCell.childNodes.length > 0, t("files.delete"));
        deleteButton.addEventListener("click", async () => {
            if (!window.confirm(t("files.deleteConfirm", { name: entry.name }))) {
                return;
            }
            await api("/api/files/delete", {
                method: "POST",
                body: JSON.stringify({ root: fileState.root, path: entry.path })
            });
            showToast(t("files.deleted", { name: entry.name }));
            await browseFiles(fileState.root, fileState.path);
        });
        actionsCell.appendChild(deleteButton);
    }

    row.append(nameCell, typeCell, sizeCell, modifiedCell, actionsCell);
    return row;
}

function prefixedActionLabel(hasPrefix, label) {
    return `${hasPrefix ? " / " : ""}${label}`;
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
    ui.toast.replaceChildren(document.createTextNode(message));
    setToastState(isError, 3600);
}

function showToastLink(message, linkLabel, url) {
    const messageNode = document.createElement("span");
    messageNode.textContent = message;

    const spacer = document.createTextNode(" ");
    const link = document.createElement("a");
    link.href = url;
    link.target = "_blank";
    link.rel = "noreferrer noopener";
    link.textContent = linkLabel;

    ui.toast.replaceChildren(messageNode, spacer, link);
    setToastState(false, 10000);
}

function setToastState(isError, durationMs) {
    ui.toast.className = `toast show${isError ? " error" : ""}`;
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(() => {
        ui.toast.className = "toast";
        ui.toast.replaceChildren();
    }, durationMs);
}

function handleError(error) {
    console.error(error);
    if (botRestartPending) {
        return;
    }
    showToast(error.message || t("common.unexpectedError"), true);
}

function escapeHtml(value) {
    return String(value)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#39;");
}
