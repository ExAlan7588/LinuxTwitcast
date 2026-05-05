window.addEventListener("DOMContentLoaded", () => {
    cacheElements();
    applyLanguage(readStoredLanguage());
    initTheme();
    initLogFilters();
    bindEvents();
    boot().catch(handleError);
});

function cacheElements() {
    cacheStatusElements();
    cacheSettingsElements();
    cacheStreamerModalElements();
    cacheNotificationElements();
    cacheFileLogElements();
    cacheActionElements();
}

function cacheStatusElements() {
    ui.statusBadge = document.getElementById("statusBadge");
    ui.recorderSummary = document.getElementById("recorderSummary");
    ui.activeRecordings = document.getElementById("activeRecordings");
    ui.manualRecordUrlInput = document.getElementById("manualRecordUrlInput");
    ui.manualRecordBtn = document.getElementById("manualRecordBtn");
    ui.runtimeInfo = document.getElementById("runtimeInfo");
    ui.diagnostics = document.getElementById("diagnostics");
}

function cacheSettingsElements() {
    ui.langInput = document.getElementById("langInput");
    ui.enableLogInput = document.getElementById("enableLogInput");
    ui.openTwitcastingAccessModalBtn = document.getElementById("openTwitcastingAccessModalBtn");
    ui.openTwitcastingAccessInlineBtn = document.getElementById("openTwitcastingAccessInlineBtn");
    ui.twitcastingAccessModal = document.getElementById("twitcastingAccessModal");
    ui.twitcastingAccessModalBackdrop = document.getElementById("twitcastingAccessModalBackdrop");
    ui.closeTwitcastingAccessModalBtn = document.getElementById("closeTwitcastingAccessModalBtn");
    ui.twitcastingApiStatusSummary = document.getElementById("twitcastingApiStatusSummary");
    ui.twitcastingCookieStatusSummary = document.getElementById("twitcastingCookieStatusSummary");
    ui.twitcastingClientIdInput = document.getElementById("twitcastingClientIdInput");
    ui.twitcastingClientSecretInput = document.getElementById("twitcastingClientSecretInput");
    ui.twitcastingCookieStatus = document.getElementById("twitcastingCookieStatus");
    ui.twitcastingCookieFileInput = document.getElementById("twitcastingCookieFileInput");
    ui.twitcastingCookieTextInput = document.getElementById("twitcastingCookieTextInput");
    ui.twitcastingCookieUploadBtn = document.getElementById("twitcastingCookieUploadBtn");
    ui.twitcastingCookieClearBtn = document.getElementById("twitcastingCookieClearBtn");
    ui.streamersBody = document.getElementById("streamersBody");
}

function cacheStreamerModalElements() {
    ui.streamerGuideBtn = document.getElementById("streamerGuideBtn");
    ui.streamerModal = document.getElementById("streamerModal");
    ui.streamerModalBackdrop = document.getElementById("streamerModalBackdrop");
    ui.streamerModalTitle = document.getElementById("streamerModalTitle");
    ui.checkStreamerModalBtn = document.getElementById("checkStreamerModalBtn");
    ui.closeStreamerModalBtn = document.getElementById("closeStreamerModalBtn");
    ui.cancelStreamerModalBtn = document.getElementById("cancelStreamerModalBtn");
    ui.saveStreamerModalBtn = document.getElementById("saveStreamerModalBtn");
    ui.streamerScreenIdInput = document.getElementById("streamerScreenIdInput");
    ui.streamerValidationStatus = document.getElementById("streamerValidationStatus");
    ui.streamerScheduleSecondsInput = document.getElementById("streamerScheduleSecondsInput");
    ui.streamerScheduleNotice = document.getElementById("streamerScheduleNotice");
    ui.streamerFolderSuffixInput = document.getElementById("streamerFolderSuffixInput");
    ui.streamerPasswordInput = document.getElementById("streamerPasswordInput");
    ui.streamerGuideModal = document.getElementById("streamerGuideModal");
    ui.streamerGuideModalBackdrop = document.getElementById("streamerGuideModalBackdrop");
    ui.closeStreamerGuideModalBtn = document.getElementById("closeStreamerGuideModalBtn");
}

function cacheNotificationElements() {
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
}

function cacheFileLogElements() {
    ui.filePathLabel = document.getElementById("filePathLabel");
    ui.fileUpBtn = document.getElementById("fileUpBtn");
    ui.filesBody = document.getElementById("filesBody");
    ui.logsPanel = document.getElementById("logsPanel");
    ui.logsSummary = document.getElementById("logsSummary");
    ui.logAlerts = document.getElementById("logAlerts");
    ui.hideOfflineLogsInput = document.getElementById("hideOfflineLogsInput");
    ui.errorsOnlyLogsInput = document.getElementById("errorsOnlyLogsInput");
    ui.checkVersionBtn = document.getElementById("checkVersionBtn");
}

function cacheActionElements() {
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

    renderTwitCastingAPIStatus();
    renderTwitCastingAuth(appState.twitcastingAuth || {});
    renderStreamers(appState.streamers);
    updateStreamerModalCopy();

    renderLogs();
}

function bindEvents() {
    ui.themeToggleBtn.addEventListener("click", toggleTheme);
    ui.refreshBtn.addEventListener("click", () => refreshAll().catch(handleError));
    ui.saveSettingsBtn.addEventListener("click", () => saveSettings().catch(handleError));
    ui.checkVersionBtn.addEventListener("click", () => checkForUpdates().catch(handleError));
    ui.openTwitcastingAccessModalBtn.addEventListener("click", openTwitcastingAccessModal);
    ui.openTwitcastingAccessInlineBtn.addEventListener("click", openTwitcastingAccessModal);
    ui.closeTwitcastingAccessModalBtn.addEventListener("click", closeTwitcastingAccessModal);
    ui.twitcastingAccessModalBackdrop.addEventListener("click", closeTwitcastingAccessModal);
    ui.twitcastingCookieUploadBtn.addEventListener("click", () => uploadTwitCastingCookie().catch(handleError));
    ui.twitcastingCookieClearBtn.addEventListener("click", () => clearTwitCastingCookie().catch(handleError));
    ui.twitcastingClientIdInput.addEventListener("input", renderTwitCastingAPIStatus);
    ui.twitcastingClientSecretInput.addEventListener("input", renderTwitCastingAPIStatus);
    ui.discordTestBtn.addEventListener("click", () => sendDiscordTest().catch(handleError));
    ui.telegramTestBtn.addEventListener("click", () => sendTelegramTest().catch(handleError));
    ui.manualRecordBtn.addEventListener("click", () => startManualRecord().catch(handleError));
    ui.startRecorderBtn.addEventListener("click", () => controlRecorder("start").catch(handleError));
    ui.stopRecorderBtn.addEventListener("click", () => controlRecorder("stop").catch(handleError));
    ui.restartRecorderBtn.addEventListener("click", () => controlRecorder("restart").catch(handleError));
    ui.restartBotBtn.addEventListener("click", () => restartBot().catch(handleError));
    ui.addStreamerBtn.addEventListener("click", () => openStreamerModal());
    ui.streamerGuideBtn.addEventListener("click", openStreamerGuideModal);
    ui.checkStreamerModalBtn.addEventListener("click", () => checkStreamerFromModal().catch(handleError));
    ui.closeStreamerModalBtn.addEventListener("click", closeStreamerModal);
    ui.cancelStreamerModalBtn.addEventListener("click", closeStreamerModal);
    ui.streamerModalBackdrop.addEventListener("click", closeStreamerModal);
    ui.saveStreamerModalBtn.addEventListener("click", () => {
        try {
            saveStreamerFromModal();
        } catch (error) {
            handleError(error);
        }
    });
    ui.streamerScreenIdInput.addEventListener("input", handleStreamerScreenIdInput);
    ui.streamerFolderSuffixInput.addEventListener("input", () => {
        streamerModalState.folderTouched = true;
    });
    ui.closeStreamerGuideModalBtn.addEventListener("click", closeStreamerGuideModal);
    ui.streamerGuideModalBackdrop.addEventListener("click", closeStreamerGuideModal);
    ui.fileUpBtn.addEventListener("click", () => browseFiles(fileState.root, parentPath(fileState.path)).catch(handleError));
    ui.fileRefreshBtn.addEventListener("click", () => browseFiles(fileState.root, fileState.path).catch(handleError));
    ui.logsRefreshBtn.addEventListener("click", () => loadLogs().catch(handleError));
    ui.hideOfflineLogsInput.addEventListener("change", handleOfflineLogFilterChange);
    ui.errorsOnlyLogsInput.addEventListener("change", handleOfflineLogFilterChange);
    ui.langInput.addEventListener("change", () => applyLanguage(ui.langInput.value));
    document.addEventListener("keydown", handleGlobalKeydown);
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
    document.documentElement.dataset.bsTheme = nextTheme;

    if (ui.themeToggleBtn) {
        const isDark = nextTheme === "dark";
        setButtonLabel(ui.themeToggleBtn, isDark ? t("theme.useLight") : t("theme.useDark"));
        ui.themeToggleBtn.setAttribute("aria-label", isDark ? t("theme.ariaLight") : t("theme.ariaDark"));
        ui.themeToggleBtn.setAttribute("aria-pressed", String(isDark));
    }

    try {
        window.localStorage.setItem(THEME_STORAGE_KEY, nextTheme);
    } catch {}
}

function initLogFilters() {
    logState.hideOffline = readStoredHideOfflineLogs();
    logState.errorsOnly = readStoredErrorsOnlyLogs();
    ui.hideOfflineLogsInput.checked = logState.hideOffline;
    ui.errorsOnlyLogsInput.checked = logState.errorsOnly;
}

function readStoredHideOfflineLogs() {
    try {
        const stored = window.localStorage.getItem(HIDE_OFFLINE_LOGS_STORAGE_KEY);
        return stored !== "0";
    } catch {
        return true;
    }
}

function readStoredErrorsOnlyLogs() {
    try {
        return window.localStorage.getItem(ERRORS_ONLY_LOGS_STORAGE_KEY) === "1";
    } catch {
        return false;
    }
}

function handleOfflineLogFilterChange() {
    logState.hideOffline = ui.hideOfflineLogsInput.checked;
    logState.errorsOnly = ui.errorsOnlyLogsInput.checked;
    try {
        window.localStorage.setItem(HIDE_OFFLINE_LOGS_STORAGE_KEY, logState.hideOffline ? "1" : "0");
        window.localStorage.setItem(ERRORS_ONLY_LOGS_STORAGE_KEY, logState.errorsOnly ? "1" : "0");
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
    const recordingsRoot = renderFileRoots(data.file_roots || []);

    if (!appState.files && recordingsRoot?.root) {
        if (recordingsRoot.exists) {
            await browseFiles(recordingsRoot.root, "");
        } else {
            renderFiles({ root: recordingsRoot.root, path: "", entries: [] });
        }
    }
}

async function loadSettings() {
    const settings = await api("/api/settings");
    renderSettings(settings);

    if (!appState.files) {
        const roots = await api("/api/files/roots");
        const recordingsRoot = renderFileRoots(roots);
        if (recordingsRoot?.root) {
            if (recordingsRoot.exists) {
                await browseFiles(recordingsRoot.root, "");
            } else {
                renderFiles({ root: recordingsRoot.root, path: "", entries: [] });
            }
        }
    }
}

async function loadLogs() {
    const params = new URLSearchParams({ limit: "250" });
    if (logState.hideOffline) {
        params.set("hide_offline", "1");
    }
    if (logState.errorsOnly) {
        params.set("errors_only", "1");
    }
    const data = await api(`/api/logs?${params.toString()}`);
    logState.lines = Array.isArray(data.lines) ? data.lines : [];
    logState.alertLines = Array.isArray(data.alert_lines) ? data.alert_lines : [];
    logState.filteredCount = Number.isFinite(data.filtered_count) ? data.filtered_count : 0;
    logState.loaded = true;
    renderLogs();
}
