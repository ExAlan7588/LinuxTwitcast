// Filter offline polling on the backend first so noisy lines do not push useful logs out of the latest window.
function renderLogs() {
    if (!logState.loaded) {
        ui.logsSummary.textContent = "";
        ui.logAlerts.textContent = t("common.loading");
        ui.logsPanel.textContent = t("common.loading");
        return;
    }

    ui.logsSummary.textContent = buildLogSummary();
    renderLogAlerts();

    if (!logState.lines.length) {
        if ((logState.hideOffline || logState.errorsOnly) && logState.filteredCount > 0) {
            ui.logsPanel.textContent = t("logs.filteredCombined", { count: logState.filteredCount });
        } else {
            ui.logsPanel.textContent = t("logs.noLines");
        }
    } else {
        ui.logsPanel.textContent = logState.lines.join("\n");
    }
    ui.logsPanel.scrollTop = ui.logsPanel.scrollHeight;
}

function buildLogSummary() {
    if (!logState.hideOffline && !logState.errorsOnly) {
        return t("logs.raw");
    }
    if (logState.hideOffline && logState.errorsOnly) {
        return logState.filteredCount > 0 ?
            t("logs.filteredCombined", { count: logState.filteredCount }) :
            `${t("logs.filterEnabled")} ${t("logs.errorsOnlyEnabled")}`;
    }
    if (logState.filteredCount <= 0) {
        return logState.hideOffline ? t("logs.filterEnabled") : t("logs.errorsOnlyEnabled");
    }
    return logState.hideOffline ?
        t("logs.filteredCount", { count: logState.filteredCount }) :
        t("logs.filteredByErrorsOnly", { count: logState.filteredCount });
}

function renderLogAlerts() {
    ui.logAlerts.innerHTML = "";
    if (!logState.alertLines.length) {
        ui.logAlerts.textContent = t("logs.noAlerts");
        return;
    }

    logState.alertLines.forEach((line) => {
        const item = document.createElement("div");
        item.className = "log-alert-item mono";
        item.textContent = line;
        ui.logAlerts.appendChild(item);
    });
}

function renderStatus(data) {
    const recorder = data.recorder || {};
    const runtime = data.runtime || {};
    const diagnostics = data.diagnostics || [];
    const activeRecordings = Array.isArray(recorder.active_recordings) ? recorder.active_recordings : [];
    const activeDownloads = Array.isArray(recorder.active_downloads) ? recorder.active_downloads : [];

    renderRecorderBadge(recorder);
    renderMetrics(recorder, activeRecordings, activeDownloads);
    renderActiveJobs(activeRecordings, activeDownloads);
    renderRuntimeInfo(runtime);
    renderDiagnostics(diagnostics);
    ui.startRecorderBtn.disabled = recorder.running || recorder.stopping;
    ui.stopRecorderBtn.disabled = !recorder.running && !recorder.stopping;
}

function renderRecorderBadge(recorder) {
    ui.statusBadge.className = `badge ${recorder.stopping ? "stopping" : recorder.running ? "running" : "stopped"}`;
    ui.statusBadge.textContent = recorder.stopping ? t("status.stopping") : recorder.running ? t("status.running") : t("status.stopped");
}

function renderMetrics(recorder, activeRecordings, activeDownloads) {
    const metrics = [
        ["people", t("metrics.enabledStreamers"), recorder.enabled_streamers ?? 0, t("streamers.passwordMissing")],
        ["calendar-check", t("metrics.scheduledJobs"), recorder.scheduled_jobs ?? 0, t("table.schedule")],
        ["arrow-down", t("metrics.activeRecordings"), activeRecordings.length + activeDownloads.length, t("status.downloading")],
        ["clock", t("metrics.uptime"), recorder.uptime || t("metrics.notStarted"), recorder.running ? t("status.running") : t("status.stopped")],
        ["activity", t("status.section"), recorder.stopping ? t("status.stopping") : recorder.running ? t("status.running") : t("status.stopped"), t("system.noDiagnostics")]
    ];
    ui.recorderSummary.innerHTML = "";
    metrics.forEach((metric) => ui.recorderSummary.appendChild(createMetricCard(metric)));

    if (recorder.last_error) {
        const item = document.createElement("div");
        item.className = "metric metric-wide";
        item.innerHTML = `<span>${t("metrics.latestError")}</span><strong>${escapeHtml(recorder.last_error)}</strong>`;
        ui.recorderSummary.appendChild(item);
    }
}

function renderActiveJobs(activeRecordings, activeDownloads) {
    ui.activeRecordings.innerHTML = "";
    if (!activeRecordings.length && !activeDownloads.length) {
        ui.activeRecordings.textContent = t("status.noActiveRecordings");
        return;
    }
    activeRecordings.forEach((entry) => ui.activeRecordings.appendChild(createActiveRecordingItem(entry)));
    activeDownloads.forEach((entry) => ui.activeRecordings.appendChild(createActiveDownloadItem(entry)));
}

function renderRuntimeInfo(runtime) {
    ui.runtimeInfo.innerHTML = "";
    [
        [t("system.version"), runtime.version || "-"],
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
}

function renderDiagnostics(diagnostics) {
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
}

function createMetricCard([icon, label, value, note]) {
    const item = document.createElement("div");
    item.className = "metric";
    item.innerHTML = `
        <div class="metric-icon"><i class="bi bi-${icon}"></i></div>
        <div>
            <span>${escapeHtml(label)}</span>
            <strong>${escapeHtml(value)}</strong>
            <small>${escapeHtml(note)}</small>
        </div>
    `;
    return item;
}

function createActiveRecordingItem(entry) {
    const item = document.createElement("div");
    item.className = "record-item";
    const badges = [];
    if (entry.member_only) {
        badges.push(`<span class="badge muted">${escapeHtml(t("status.memberOnly"))}</span>`);
    }
    if (entry.movie_id) {
        badges.push(`<span class="badge muted mono">${escapeHtml(entry.movie_id)}</span>`);
    }
    item.innerHTML = `
        <div><strong>${escapeHtml(entry.streamer_name || entry.streamer)}</strong> ${badges.join(" ")}</div>
        <div class="muted-text mono">${escapeHtml(entry.streamer)}</div>
        <div>${escapeHtml(entry.title || t("status.untitledStream"))}</div>
        ${entry.movie_id ? `<div class="muted-text mono">${escapeHtml(t("status.movieId", { id: entry.movie_id }))}</div>` : ""}
        <div class="muted-text mono">${escapeHtml(entry.filename || "")}</div>
    `;
    return item;
}

function createActiveDownloadItem(entry) {
    const item = document.createElement("div");
    const progress = clampProgressPercent(entry.progress_percent);
    const currentPart = Number.isFinite(entry.current_part) ? Math.max(0, Math.trunc(entry.current_part)) : 0;
    const totalParts = Number.isFinite(entry.total_parts) ? Math.max(0, Math.trunc(entry.total_parts)) : 0;
    const statusLabel = totalParts > 0 ?
        `${t("status.downloading")} · ${t("status.downloadPart", { current: Math.max(currentPart, 1), total: totalParts })}` :
        t("status.downloading");

    item.className = "record-item download-item";
    item.innerHTML = `
        <strong>${escapeHtml(entry.streamer_name || entry.streamer)}</strong>
        <div class="muted-text mono">${escapeHtml(entry.streamer || "")}${entry.movie_id ? ` · movie/${escapeHtml(entry.movie_id)}` : ""}</div>
        <div>${escapeHtml(entry.title || t("status.untitledStream"))}</div>
        <div class="muted-text mono">${escapeHtml(entry.current_file || "")}</div>
        <div class="download-progress-meta">
            <span>${escapeHtml(statusLabel)}</span>
            <span class="mono">${progress.toFixed(0)}%</span>
        </div>
        <div class="download-progress-track" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${progress.toFixed(0)}">
            <div class="download-progress-bar" style="width: ${progress.toFixed(2)}%;"></div>
        </div>
    `;
    return item;
}

function clampProgressPercent(value) {
    const numeric = Number(value);
    if (!Number.isFinite(numeric) || numeric < 0) {
        return 0;
    }
    if (numeric > 100) {
        return 100;
    }
    return numeric;
}
