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

async function uploadTwitCastingCookie() {
    await runButtonAction(ui.twitcastingCookieUploadBtn, t("twitcasting.cookieUploading"), async () => {
        const file = ui.twitcastingCookieFileInput.files?.[0];
        const content = file ? await file.text() : ui.twitcastingCookieTextInput.value.trim();
        if (!content) {
            throw new Error(t("twitcasting.cookieRequired"));
        }

        const result = await api("/api/twitcasting/auth", {
            method: "PUT",
            body: JSON.stringify({ content })
        });

        ui.twitcastingCookieFileInput.value = "";
        ui.twitcastingCookieTextInput.value = "";
        appState.twitcastingAuth = result.status || {};
        renderTwitCastingAuth(appState.twitcastingAuth);
        showToast(t("twitcasting.cookieSaved"));
    });
}

async function clearTwitCastingCookie() {
    await runButtonAction(ui.twitcastingCookieClearBtn, t("twitcasting.cookieClearing"), async () => {
        const result = await api("/api/twitcasting/auth", { method: "DELETE" });
        ui.twitcastingCookieFileInput.value = "";
        ui.twitcastingCookieTextInput.value = "";
        appState.twitcastingAuth = result.status || {};
        renderTwitCastingAuth(appState.twitcastingAuth);
        showToast(t("twitcasting.cookieCleared"));
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
    const originalLabel = getButtonLabel(button);
    button.disabled = true;
    setButtonLabel(button, busyLabel);
    try {
        await action();
    } finally {
        button.disabled = false;
        setButtonLabel(button, originalLabel);
    }
}

function getButtonLabel(button) {
    return button.querySelector("[data-button-label]")?.textContent || button.textContent;
}

function setButtonLabel(button, label) {
    const labelNode = button.querySelector("[data-button-label]");
    if (labelNode) {
        labelNode.textContent = label;
        return;
    }
    button.textContent = label;
}

function collectSettings() {
    const app = collectAppSettings();
    const discord = collectDiscordSettings();
    const telegram = collectTelegramSettings();

    return { app, discord, telegram };
}

function collectAppSettings() {
    const streamers = appState.streamers.map((streamer) => ({
        enabled: streamer.enabled,
        "screen-id": streamer["screen-id"],
        schedule: streamer.schedule,
        folder: streamer.folder,
        password: streamer.password
    }));

    return {
        lang: ui.langInput.value,
        enable_log: ui.enableLogInput.checked,
        twitcasting_api: {
            client_id: ui.twitcastingClientIdInput.value.trim(),
            client_secret: ui.twitcastingClientSecretInput.value.trim()
        },
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

async function startManualRecord() {
    await runButtonAction(ui.manualRecordBtn, t("manual.starting"), async () => {
        const rawURL = ui.manualRecordUrlInput.value.trim();
        if (!rawURL) {
            throw new Error(t("manual.urlRequired"));
        }

        const result = await api("/api/manual/record", {
            method: "POST",
            body: JSON.stringify({ url: rawURL })
        });

        ui.manualRecordUrlInput.value = "";
        const toastKey = result.mode === "download" ? "manual.startedDownload" : "manual.startedRecord";
        showToast(t(toastKey, {
            name: result.name || result.streamer || "—",
            title: result.title || t("status.untitledStream")
        }));
        await Promise.all([loadStatus(), loadLogs()]);
    });
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
