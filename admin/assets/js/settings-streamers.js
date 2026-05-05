function renderSettings(settings) {
    appState.streamers = normalizeStreamers(settings.app?.streamers || []);
    appState.twitcastingAuth = settings.twitcasting_auth || {};
    ui.langInput.value = settings.app?.lang || "EN";
    applyLanguage(ui.langInput.value);
    ui.enableLogInput.checked = Boolean(settings.app?.enable_log);
    ui.twitcastingClientIdInput.value = settings.app?.twitcasting_api?.client_id || "";
    ui.twitcastingClientSecretInput.value = settings.app?.twitcasting_api?.client_secret || "";
    renderTwitCastingAPIStatus();
    renderTwitCastingAuth(appState.twitcastingAuth);
    renderStreamers(appState.streamers);

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

function renderTwitCastingAuth(status) {
    const configured = Boolean(status?.configured);
    const cookieCount = Number.isFinite(status?.cookie_count) ? status.cookie_count : 0;
    ui.twitcastingCookieStatus.className = `badge ${configured ? "running" : "muted"}`;
    ui.twitcastingCookieStatus.textContent = configured ?
        t("twitcasting.cookieConfigured", { count: cookieCount }) :
        t("twitcasting.cookieMissing");
    ui.twitcastingCookieStatusSummary.className = `badge ${configured ? "running" : "muted"}`;
    ui.twitcastingCookieStatusSummary.textContent = configured ?
        t("twitcasting.cookieConfigured", { count: cookieCount }) :
        t("twitcasting.cookieMissing");
    ui.twitcastingCookieClearBtn.disabled = !configured;
}

function renderTwitCastingAPIStatus() {
    const clientID = ui.twitcastingClientIdInput?.value.trim() || "";
    const clientSecret = ui.twitcastingClientSecretInput?.value.trim() || "";

    let tone = "muted";
    let key = "settings.twitcastingApiMissing";
    if (clientID && clientSecret) {
        tone = "running";
        key = "settings.twitcastingApiConfigured";
    } else if (clientID || clientSecret) {
        tone = "stopping";
        key = "settings.twitcastingApiIncomplete";
    }

    ui.twitcastingApiStatusSummary.className = `badge ${tone}`;
    ui.twitcastingApiStatusSummary.textContent = t(key);
}

function renderStreamers(streamers) {
    ui.streamersBody.innerHTML = "";

    if (!streamers.length) {
        const row = document.createElement("tr");
        row.innerHTML = `<td colspan="6" class="muted-text">${escapeHtml(t("streamers.none"))}</td>`;
        ui.streamersBody.appendChild(row);
        return;
    }

    streamers.forEach((streamer, index) => {
        ui.streamersBody.appendChild(createStreamerRow(streamer, index));
    });
}

function createStreamerRow(streamer = {}, index) {
    const row = document.createElement("tr");
    const enabledCell = document.createElement("td");
    const screenIdCell = document.createElement("td");
    const scheduleCell = document.createElement("td");
    const passwordCell = document.createElement("td");
    const folderCell = document.createElement("td");
    const actionsCell = document.createElement("td");
    actionsCell.className = "actions-cell";
    screenIdCell.className = "mono";
    folderCell.className = "mono";

    const enabledButton = document.createElement("button");
    enabledButton.type = "button";
    enabledButton.className = `streamer-toggle ${streamer.enabled ? "enabled" : "disabled"}`;
    enabledButton.textContent = streamer.enabled ? t("common.enabled") : t("common.disabled");
    enabledButton.addEventListener("click", () => toggleStreamerEnabled(index));
    enabledCell.appendChild(enabledButton);

    const editButton = document.createElement("button");
    editButton.type = "button";
    editButton.className = "table-link table-action";
    editButton.textContent = t("streamers.edit");
    editButton.addEventListener("click", () => openStreamerModal(index));

    const removeButton = document.createElement("button");
    removeButton.type = "button";
    removeButton.className = "table-link table-action danger-link";
    removeButton.textContent = prefixedActionLabel(true, t("streamers.remove"));
    removeButton.addEventListener("click", () => removeStreamer(index));

    actionsCell.append(editButton, removeButton);
    screenIdCell.textContent = streamer["screen-id"] || "—";
    scheduleCell.textContent = formatScheduleDisplay(streamer.schedule);
    passwordCell.textContent = streamer.password ? t("streamers.passwordSet") : t("streamers.passwordMissing");
    folderCell.textContent = formatStreamerFolder(streamer.folder, streamer["screen-id"]);

    row.append(enabledCell, screenIdCell, scheduleCell, passwordCell, folderCell, actionsCell);

    return row;
}

function normalizeStreamers(streamers) {
    if (!Array.isArray(streamers)) {
        return [];
    }
    return streamers
        .filter(Boolean)
        .map((streamer) => ({
            enabled: Boolean(streamer.enabled),
            "screen-id": String(streamer["screen-id"] || "").trim(),
            schedule: String(streamer.schedule || "@every 5s").trim() || "@every 5s",
            password: String(streamer.password || "").trim(),
            folder: buildStreamerFolder(extractStreamerFolderSuffix(streamer.folder || "", streamer["screen-id"] || ""))
        }));
}

function openStreamerModal(index = null) {
    const isEdit = Number.isInteger(index);
    const streamer = isEdit ? appState.streamers[index] : defaultStreamerEntry();
    streamerModalState.index = isEdit ? index : null;
    streamerModalState.folderTouched = isEdit;
    streamerModalState.rawSchedule = streamer.schedule && scheduleToSeconds(streamer.schedule) === null ? streamer.schedule : "";
    streamerModalState.initialScreenId = String(streamer["screen-id"] || "").trim();
    streamerModalState.verifiedScreenId = isEdit ? streamerModalState.initialScreenId : "";

    ui.streamerScreenIdInput.value = streamer["screen-id"] || "";
    ui.streamerScheduleSecondsInput.value = String(scheduleToSeconds(streamer.schedule) || 5);
    ui.streamerFolderSuffixInput.value = extractStreamerFolderSuffix(streamer.folder, streamer["screen-id"]);
    ui.streamerPasswordInput.value = streamer.password || "";
    setStreamerValidationState(isEdit ? "muted" : "warn", isEdit ? "" : "streamer.checkNeededAdd");

    updateStreamerModalCopy();
    openModalShell(ui.streamerModal, ui.streamerScreenIdInput);
}

function closeStreamerModal() {
    streamerModalState.index = null;
    streamerModalState.folderTouched = false;
    streamerModalState.rawSchedule = "";
    streamerModalState.initialScreenId = "";
    streamerModalState.verifiedScreenId = "";
    setStreamerValidationState("muted", "");
    closeModalShell(ui.streamerModal);
}

function openStreamerGuideModal() {
    openModalShell(ui.streamerGuideModal, ui.closeStreamerGuideModalBtn);
}

function closeStreamerGuideModal() {
    closeModalShell(ui.streamerGuideModal);
}

function openTwitcastingAccessModal() {
    openModalShell(ui.twitcastingAccessModal, ui.twitcastingClientIdInput);
}

function closeTwitcastingAccessModal() {
    closeModalShell(ui.twitcastingAccessModal);
}

function openModalShell(element, focusTarget) {
    element.hidden = false;
    document.body.classList.add("modal-open");
    window.setTimeout(() => focusTarget?.focus(), 0);
}

function closeModalShell(element) {
    element.hidden = true;
    if (!document.querySelector(".modal-shell:not([hidden])")) {
        document.body.classList.remove("modal-open");
    }
}

function updateStreamerModalCopy() {
    const isEdit = Number.isInteger(streamerModalState.index);
    ui.streamerModalTitle.textContent = isEdit ? t("streamer.modalTitleEdit") : t("streamer.modalTitleAdd");
    ui.saveStreamerModalBtn.textContent = isEdit ? t("streamer.modalSaveEdit") : t("streamer.modalSaveAdd");
    if (streamerModalState.rawSchedule) {
        ui.streamerScheduleNotice.hidden = false;
        ui.streamerScheduleNotice.textContent = t("streamers.customScheduleNotice", { schedule: streamerModalState.rawSchedule });
    } else {
        ui.streamerScheduleNotice.hidden = true;
        ui.streamerScheduleNotice.textContent = "";
    }
    renderStreamerValidationState();
    updateStreamerSaveState();
}

function saveStreamerFromModal() {
    const screenId = ui.streamerScreenIdInput.value.trim();
    if (!screenId) {
        throw new Error(t("streamer.requiredScreenId"));
    }
    if (!canSaveStreamerFromModal()) {
        throw new Error(t(Number.isInteger(streamerModalState.index) ? "streamer.checkNeededEdit" : "streamer.checkNeededAdd"));
    }

    const seconds = Number.parseInt(ui.streamerScheduleSecondsInput.value, 10);
    if (!Number.isFinite(seconds) || seconds < 1) {
        throw new Error(t("streamer.invalidInterval"));
    }

    const folderSuffix = (ui.streamerFolderSuffixInput.value.trim() || screenId).replace(/^\/+|\/+$/g, "");
    const currentStreamer = Number.isInteger(streamerModalState.index) ? appState.streamers[streamerModalState.index] : null;
    const nextStreamer = {
        enabled: currentStreamer ? Boolean(currentStreamer.enabled) : true,
        "screen-id": screenId,
        schedule: buildEverySchedule(seconds),
        folder: buildStreamerFolder(folderSuffix),
        password: ui.streamerPasswordInput.value.trim()
    };

    if (Number.isInteger(streamerModalState.index)) {
        appState.streamers[streamerModalState.index] = nextStreamer;
    } else {
        appState.streamers.push(nextStreamer);
    }

    renderStreamers(appState.streamers);
    closeStreamerModal();
    showToast(t("streamers.pendingSave"));
}

function removeStreamer(index) {
    const streamer = appState.streamers[index];
    if (!streamer) {
        return;
    }
    if (!window.confirm(t("streamers.removeConfirm", { id: streamer["screen-id"] || "—" }))) {
        return;
    }
    appState.streamers.splice(index, 1);
    renderStreamers(appState.streamers);
    showToast(t("streamers.pendingSave"));
}

function toggleStreamerEnabled(index) {
    const streamer = appState.streamers[index];
    if (!streamer) {
        return;
    }
    streamer.enabled = !streamer.enabled;
    renderStreamers(appState.streamers);
    showToast(t("streamers.pendingSave"));
}

function defaultStreamerEntry() {
    return {
        enabled: true,
        "screen-id": "",
        schedule: "@every 5s",
        folder: buildStreamerFolder(""),
        password: ""
    };
}

function setStreamerValidationState(tone, key, params = {}) {
    streamerModalState.validationTone = tone;
    streamerModalState.validationKey = key;
    streamerModalState.validationParams = params;
}

function renderStreamerValidationState() {
    const hasMessage = Boolean(streamerModalState.validationKey);
    ui.streamerValidationStatus.hidden = !hasMessage;
    ui.streamerValidationStatus.className = `field-help validation-status${hasMessage && streamerModalState.validationTone !== "muted" ? ` is-${streamerModalState.validationTone}` : ""}`;
    ui.streamerValidationStatus.textContent = hasMessage ? t(streamerModalState.validationKey, streamerModalState.validationParams) : "";
}

function canSaveStreamerFromModal() {
    const screenId = ui.streamerScreenIdInput.value.trim();
    if (!screenId) {
        return false;
    }
    if (Number.isInteger(streamerModalState.index) && screenId === streamerModalState.initialScreenId) {
        return true;
    }
    return screenId === streamerModalState.verifiedScreenId;
}

function updateStreamerSaveState() {
    ui.saveStreamerModalBtn.disabled = !canSaveStreamerFromModal();
}

// Screen ID 一改就重置校验，避免旧的检查结果被误用到新目标上。
function handleStreamerScreenIdInput() {
    const screenId = ui.streamerScreenIdInput.value.trim();
    if (Number.isInteger(streamerModalState.index) && screenId === streamerModalState.initialScreenId) {
        streamerModalState.verifiedScreenId = streamerModalState.initialScreenId;
        setStreamerValidationState("muted", "");
        updateStreamerSaveState();
        return;
    }

    streamerModalState.verifiedScreenId = "";
    setStreamerValidationState("warn", "streamer.checkNeededAdd");
    if (Number.isInteger(streamerModalState.index)) {
        setStreamerValidationState("warn", "streamer.checkNeededEdit");
    }
    updateStreamerSaveState();
}

async function checkStreamerFromModal() {
    const screenId = ui.streamerScreenIdInput.value.trim();
    if (!screenId) {
        throw new Error(t("streamer.requiredScreenId"));
    }

    setStreamerValidationState("muted", "streamer.checking");
    renderStreamerValidationState();

    await runButtonAction(ui.checkStreamerModalBtn, t("streamer.checking"), async () => {
        try {
            const result = await api("/api/streamers/check", {
                method: "POST",
                body: JSON.stringify({ screen_id: screenId })
            });

            streamerModalState.verifiedScreenId = screenId;
            const streamerName = sanitizeFolderSuffix(result.streamer_name || screenId) || screenId;
            if (!streamerModalState.folderTouched || !ui.streamerFolderSuffixInput.value.trim()) {
                ui.streamerFolderSuffixInput.value = streamerName;
            }
            setStreamerValidationState(
                "success",
                result.password_required ? "streamer.checkOkPassword" : "streamer.checkOk",
                { name: streamerName }
            );
            updateStreamerSaveState();
        } catch (error) {
            streamerModalState.verifiedScreenId = "";
            setStreamerValidationState("error", "streamer.checkFailed", { message: error.message || t("common.requestFailed") });
            updateStreamerSaveState();
            throw error;
        }
    });
}

function sanitizeFolderSuffix(value) {
    return String(value || "").trim().replace(/^\/+|\/+$/g, "");
}

function buildStreamerFolder(folderSuffix) {
    return `${STREAMER_FOLDER_PREFIX}${sanitizeFolderSuffix(folderSuffix)}`;
}

function extractStreamerFolderSuffix(folder, screenId) {
    const value = String(folder || "").trim();
    if (!value) {
        return String(screenId || "").trim();
    }
    if (value.startsWith(STREAMER_FOLDER_PREFIX)) {
        return value.slice(STREAMER_FOLDER_PREFIX.length);
    }
    return value.replace(/^\/+|\/+$/g, "");
}

function formatStreamerFolder(folder, screenId) {
    return buildStreamerFolder(extractStreamerFolderSuffix(folder, screenId));
}

function buildEverySchedule(seconds) {
    return `@every ${seconds}s`;
}

function scheduleToSeconds(schedule) {
    const raw = String(schedule || "").trim();
    if (!raw.toLowerCase().startsWith("@every ")) {
        return null;
    }
    const duration = raw.slice(7).trim().toLowerCase();
    const matches = Array.from(duration.matchAll(/(\d+)(h|m|s)/g));
    if (!matches.length) {
        return null;
    }

    let consumed = "";
    let totalSeconds = 0;
    matches.forEach((match) => {
        consumed += match[0];
        const amount = Number.parseInt(match[1], 10);
        switch (match[2]) {
        case "h":
            totalSeconds += amount * 3600;
            break;
        case "m":
            totalSeconds += amount * 60;
            break;
        case "s":
            totalSeconds += amount;
            break;
        }
    });

    if (consumed !== duration || totalSeconds < 1) {
        return null;
    }
    return totalSeconds;
}

function formatScheduleDisplay(schedule) {
    const seconds = scheduleToSeconds(schedule);
    if (seconds === null) {
        return String(schedule || "—");
    }
    return `${seconds} ${t("common.seconds")}`;
}

function handleGlobalKeydown(event) {
    if (event.key !== "Escape") {
        return;
    }
    if (!ui.twitcastingAccessModal.hidden) {
        closeTwitcastingAccessModal();
        return;
    }
    if (!ui.streamerModal.hidden) {
        closeStreamerModal();
        return;
    }
    if (!ui.streamerGuideModal.hidden) {
        closeStreamerGuideModal();
    }
}
