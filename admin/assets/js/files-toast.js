function renderFileRoots(roots) {
    if (!Array.isArray(roots) || !roots.length) {
        return null;
    }
    const recordingsRoot = roots.find((root) => root.label === RECORDINGS_ROOT_LABEL) ||
        roots.find((root) => /[\\/]Recordings$/i.test(String(root.root || "")));
    if (!recordingsRoot) {
        return null;
    }
    fileState.root = recordingsRoot.root;
    return recordingsRoot;
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
    ui.filePathLabel.textContent = buildFileManagerPath(fileState.path);
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

    fillFileNameCell(nameCell, entry);
    typeCell.textContent = entry.type;
    sizeCell.textContent = entry.type === "dir" ? "—" : formatBytes(entry.size || 0);
    sizeCell.className = "mono";
    modifiedCell.textContent = formatDate(entry.modified_at);

    if (entry.downloadable) {
        appendDownloadableFileActions(actionsCell, entry);
    }

    if (entry.deletable) {
        appendDeleteFileAction(actionsCell, entry);
    }

    row.append(nameCell, typeCell, sizeCell, modifiedCell, actionsCell);
    return row;
}

function fillFileNameCell(nameCell, entry) {
    if (entry.type !== "dir") {
        const span = document.createElement("span");
        span.textContent = entry.name;
        nameCell.appendChild(span);
        return;
    }
    const button = document.createElement("button");
    button.type = "button";
    button.className = "table-link table-action";
    button.textContent = entry.name;
    button.addEventListener("click", () => browseFiles(fileState.root, entry.path).catch(handleError));
    nameCell.appendChild(button);
}

function appendDownloadableFileActions(actionsCell, entry) {
    if (isM4AConvertibleFileEntry(entry)) {
        actionsCell.appendChild(createConvertFileButton(actionsCell, entry));
    }
    actionsCell.appendChild(createUploadFileButton(actionsCell, entry));
    actionsCell.appendChild(createDownloadFileLink(actionsCell, entry));
}

function createConvertFileButton(actionsCell, entry) {
    const button = createFileActionButton(actionsCell, t("files.convert"));
    button.addEventListener("click", async () => convertFileEntry(button, actionsCell, entry));
    return button;
}

function createUploadFileButton(actionsCell, entry) {
    const button = createFileActionButton(actionsCell, t("files.upload"));
    button.addEventListener("click", async () => uploadFileEntry(button, actionsCell, entry));
    return button;
}

function createFileActionButton(actionsCell, label) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "table-link table-action";
    button.textContent = prefixedActionLabel(actionsCell.childNodes.length > 0, label);
    return button;
}

function createDownloadFileLink(actionsCell, entry) {
    const link = document.createElement("a");
    link.className = "table-link";
    link.textContent = prefixedActionLabel(actionsCell.childNodes.length > 0, t("files.download"));
    link.href = `/api/files/download?root=${encodeURIComponent(fileState.root)}&path=${encodeURIComponent(entry.path)}`;
    link.setAttribute("download", "");
    return link;
}

async function convertFileEntry(button, actionsCell, entry) {
    if (!window.confirm(t("files.convertConfirm", { name: entry.name }))) {
        return;
    }
    const originalLabel = button.textContent;
    button.disabled = true;
    button.textContent = prefixedActionLabel(actionsCell.childNodes.length > 0, t("files.converting"));
    try {
        const result = await api("/api/files/convert-m4a", {
            method: "POST",
            body: JSON.stringify({ root: fileState.root, path: entry.path })
        });
        showToast(t("files.converted", { name: entry.name, output: result.output || `${entry.name}.m4a` }));
        await browseFiles(fileState.root, fileState.path);
    } finally {
        button.disabled = false;
        button.textContent = originalLabel;
    }
}

async function uploadFileEntry(button, actionsCell, entry) {
    if (!window.confirm(t("files.uploadConfirm", { name: entry.name }))) {
        return;
    }
    const originalLabel = button.textContent;
    button.disabled = true;
    button.textContent = prefixedActionLabel(actionsCell.childNodes.length > 0, t("files.uploading"));
    try {
        const result = await api("/api/files/telegram-upload", {
            method: "POST",
            body: JSON.stringify({ root: fileState.root, path: entry.path })
        });
        const modeLabel = result.method === "audio" ? t("files.methodAudio") : t("files.methodDocument");
        showToast(t("files.uploaded", { name: entry.name, mode: modeLabel }));
    } finally {
        button.disabled = false;
        button.textContent = originalLabel;
    }
}

function appendDeleteFileAction(actionsCell, entry) {
    const deleteButton = document.createElement("button");
    deleteButton.type = "button";
    deleteButton.className = "table-link table-action danger-link";
    deleteButton.textContent = prefixedActionLabel(actionsCell.childNodes.length > 0, t("files.delete"));
    deleteButton.addEventListener("click", async () => deleteFileEntry(entry));
    actionsCell.appendChild(deleteButton);
}

async function deleteFileEntry(entry) {
    const confirmMessage = entry.type === "dir" ?
        t("files.deleteRecursiveConfirm", { name: entry.name }) :
        t("files.deleteConfirm", { name: entry.name });
    if (!window.confirm(confirmMessage)) {
        return;
    }
    await api("/api/files/delete", {
        method: "POST",
        body: JSON.stringify({ root: fileState.root, path: entry.path })
    });
    showToast(t("files.deleted", { name: entry.name }));
    await browseFiles(fileState.root, fileState.path);
}

function isM4AConvertibleFileEntry(entry) {
    return entry.type === "file" && /\.(ts|mp4)$/i.test(String(entry.name || ""));
}

function prefixedActionLabel(hasPrefix, label) {
    return `${hasPrefix ? " / " : ""}${label}`;
}

function buildFileManagerPath(path) {
    const relative = String(path || "").replace(/^\/+|\/+$/g, "");
    return relative ? `${RECORDINGS_DISPLAY_ROOT}/${relative}` : RECORDINGS_DISPLAY_ROOT;
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
    ui.toast.className = `toast-message show${isError ? " error" : ""}`;
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(() => {
        ui.toast.className = "toast-message";
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
