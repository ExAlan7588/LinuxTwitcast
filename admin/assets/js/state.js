const ui = {};
const fileState = { root: "", path: "" };
const THEME_STORAGE_KEY = "twitcast-theme-v2";
const HIDE_OFFLINE_LOGS_STORAGE_KEY = "twitcast-hide-offline-logs";
const ERRORS_ONLY_LOGS_STORAGE_KEY = "twitcast-errors-only-logs";
const LANG_STORAGE_KEY = "twitcast-ui-lang";
const langState = { value: "EN" };
const themeState = { value: "dark" };
const logState = { lines: [], alertLines: [], hideOffline: true, errorsOnly: false, filteredCount: 0, loaded: false };
const appState = { status: null, files: null, streamers: [], twitcastingAuth: {} };
const streamerModalState = {
    index: null,
    folderTouched: false,
    rawSchedule: "",
    initialScreenId: "",
    verifiedScreenId: "",
    validationTone: "muted",
    validationKey: "",
    validationParams: {}
};
const RECORDINGS_ROOT_LABEL = "Recordings";
const RECORDINGS_DISPLAY_ROOT = "/Recordings";
const STREAMER_FOLDER_PREFIX = "Recordings/";
let botRestartPending = false;
let toastTimer = null;
