package admin

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/applog"
	"github.com/jzhang046/croned-twitcasting-recorder/config"
	"github.com/jzhang046/croned-twitcasting-recorder/discord"
	"github.com/jzhang046/croned-twitcasting-recorder/service"
	"github.com/jzhang046/croned-twitcasting-recorder/telegram"
	"github.com/jzhang046/croned-twitcasting-recorder/twitcasting"
)

//go:embed assets/*
var assets embed.FS

var lookupStreamerProfile = twitcasting.LookupStreamerProfile

type Options struct {
	Address  string
	RootDir  string
	Username string
	Password string
}

type Server struct {
	options          Options
	manager          *service.Manager
	restartRequested chan<- struct{}
	httpServer       *http.Server
	buildInfo        BuildInfo
	fileRoots        []FileRoot
	fileRootByPath   map[string]string
	recordingsRoot   string
	executablePath   string
	ffmpegPath       string
}

type RuntimeInfo struct {
	Version          string `json:"version"`
	GitCommit        string `json:"git_commit,omitempty"`
	OS               string `json:"os"`
	Arch             string `json:"arch"`
	WorkingDirectory string `json:"working_directory"`
	Executable       string `json:"executable"`
	FFmpegPath       string `json:"ffmpeg_path,omitempty"`
	ListenAddress    string `json:"listen_address"`
	AuthEnabled      bool   `json:"auth_enabled"`
}

type Diagnostic struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type FileEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Type         string `json:"type"`
	Size         int64  `json:"size"`
	ModifiedAt   string `json:"modified_at"`
	Downloadable bool   `json:"downloadable"`
	Deletable    bool   `json:"deletable"`
}

type FileListResponse struct {
	Root    string      `json:"root"`
	Path    string      `json:"path"`
	Parent  string      `json:"parent,omitempty"`
	Entries []FileEntry `json:"entries"`
}

func NewServer(options Options, manager *service.Manager, restartRequested chan<- struct{}) *Server {
	executable, _ := os.Executable()
	ffmpegPath, _ := exec.LookPath("ffmpeg")
	fileRoots := BuildFileRoots(options.RootDir)
	fileRootByPath := make(map[string]string, len(fileRoots)*2)
	recordingsRoot := ""
	for _, root := range fileRoots {
		rootAbs, err := filepath.Abs(filepath.Clean(root.Root))
		if err != nil {
			rootAbs = filepath.Clean(root.Root)
		}
		rootAbs = filepath.Clean(rootAbs)

		cleanRoot := filepath.Clean(root.Root)
		fileRootByPath[cleanRoot] = rootAbs
		fileRootByPath[rootAbs] = rootAbs
		if recordingsRoot == "" {
			recordingsRoot = rootAbs
		}
	}

	server := &Server{
		options:          options,
		manager:          manager,
		restartRequested: restartRequested,
		buildInfo:        LoadBuildInfo(options.RootDir),
		executablePath:   executable,
		ffmpegPath:       ffmpegPath,
		fileRoots:        fileRoots,
		fileRootByPath:   fileRootByPath,
		recordingsRoot:   recordingsRoot,
	}

	mux := http.NewServeMux()
	mux.Handle("/", server.withAuth(http.HandlerFunc(server.handleIndex)))
	mux.Handle("/assets/", server.withAuth(server.assetHandler()))
	mux.Handle("/api/status", server.withAuth(http.HandlerFunc(server.handleStatus)))
	mux.Handle("/api/version/check", server.withAuth(http.HandlerFunc(server.handleVersionCheck)))
	mux.Handle("/api/settings", server.withAuth(http.HandlerFunc(server.handleSettings)))
	mux.Handle("/api/streamers/check", server.withAuth(http.HandlerFunc(server.handleStreamerCheck)))
	mux.Handle("/api/discord/test", server.withAuth(http.HandlerFunc(server.handleDiscordTest)))
	mux.Handle("/api/telegram/test", server.withAuth(http.HandlerFunc(server.handleTelegramTest)))
	mux.Handle("/api/recorder/start", server.withAuth(http.HandlerFunc(server.handleRecorderStart)))
	mux.Handle("/api/recorder/stop", server.withAuth(http.HandlerFunc(server.handleRecorderStop)))
	mux.Handle("/api/recorder/restart", server.withAuth(http.HandlerFunc(server.handleRecorderRestart)))
	mux.Handle("/api/bot/restart", server.withAuth(http.HandlerFunc(server.handleBotRestart)))
	mux.Handle("/api/logs", server.withAuth(http.HandlerFunc(server.handleLogs)))
	mux.Handle("/api/files/roots", server.withAuth(http.HandlerFunc(server.handleFileRoots)))
	mux.Handle("/api/files", server.withAuth(http.HandlerFunc(server.handleFiles)))
	mux.Handle("/api/files/download", server.withAuth(http.HandlerFunc(server.handleFileDownload)))
	mux.Handle("/api/files/telegram-upload", server.withAuth(http.HandlerFunc(server.handleFileTelegramUpload)))
	mux.Handle("/api/files/delete", server.withAuth(http.HandlerFunc(server.handleFileDelete)))

	server.httpServer = &http.Server{
		Addr:    options.Address,
		Handler: mux,
	}

	return server
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	content, err := assets.ReadFile("assets/index.html")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	appConfig, telegramCfg, err := LoadRuntimeDiagnosticsConfig()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	executable := s.executablePath
	ffmpegPath := s.ffmpegPath
	status := s.manager.Status()

	s.writeJSON(w, map[string]interface{}{
		"recorder": status,
		"runtime": RuntimeInfo{
			Version:          s.buildInfo.Version,
			GitCommit:        s.buildInfo.ShortCommit,
			OS:               runtime.GOOS,
			Arch:             runtime.GOARCH,
			WorkingDirectory: s.options.RootDir,
			Executable:       executable,
			FFmpegPath:       ffmpegPath,
			ListenAddress:    s.options.Address,
			AuthEnabled:      s.options.Username != "",
		},
		"file_roots":    s.fileRoots,
		"diagnostics":   s.buildDiagnostics(appConfig, telegramCfg, status, ffmpegPath),
		"needs_restart": status.Running,
	})
}

func (s *Server) handleVersionCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	s.writeJSON(w, CheckForUpdates(s.options.RootDir, s.buildInfo))
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := LoadSettings()
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, settings)
	case http.MethodPut:
		var settings Settings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := SaveSettings(settings); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		warning := ""
		if err := applog.Configure(settings.App.EnableLog); err != nil {
			warning = fmt.Sprintf("settings were saved, but app.log could not be opened: %v", err)
		}
		s.writeJSON(w, map[string]interface{}{
			"saved":         true,
			"needs_restart": s.manager.Status().Running,
			"warning":       warning,
		})
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleStreamerCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	var req struct {
		ScreenID string `json:"screen_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	screenID := strings.TrimSpace(req.ScreenID)
	if screenID == "" {
		s.writeError(w, http.StatusBadRequest, errors.New("screen-id is required"))
		return
	}

	// 这里只检查主页是否存在并能解析主播资料，避免把“未开播”误判成无效 screen-id。
	profile, err := lookupStreamerProfile(screenID)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, twitcasting.ErrStreamerNotFound) {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "screen-id is required") {
			status = http.StatusBadRequest
		}
		s.writeError(w, status, err)
		return
	}

	s.writeJSON(w, map[string]any{
		"ok":                true,
		"screen_id":         profile.ScreenID,
		"streamer_name":     profile.StreamerName,
		"title":             profile.Title,
		"avatar_url":        profile.AvatarURL,
		"password_required": profile.PasswordRequired,
	})
}

// Test notifications read the temporary request payload so credentials can be checked before saving to disk.
func (s *Server) handleDiscordTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	var cfg discord.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := discord.SendTestMessage(cfg, fmt.Sprintf("LinuxTwitcast Discord test message\nTime: %s\nSource: Web console", time.Now().Format(time.RFC3339))); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}

	s.writeJSON(w, map[string]any{
		"sent":       true,
		"channel_id": strings.TrimSpace(cfg.NotifyChannelID),
	})
}

// Test notifications read the temporary request payload so credentials can be checked before saving to disk.
func (s *Server) handleTelegramTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	var cfg telegram.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := telegram.SendTestMessage(cfg, fmt.Sprintf("LinuxTwitcast Telegram test message\nTime: %s\nSource: Web console", time.Now().Format(time.RFC3339))); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}

	s.writeJSON(w, map[string]any{
		"sent":    true,
		"chat_id": strings.TrimSpace(cfg.ChatID),
	})
}

func (s *Server) handleRecorderStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	if err := s.manager.Start(); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, map[string]any{"status": s.manager.Status()})
}

func (s *Server) handleRecorderStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.manager.Stop(ctx); err != nil {
		s.writeError(w, http.StatusRequestTimeout, err)
		return
	}
	s.writeJSON(w, map[string]any{"status": s.manager.Status()})
}

func (s *Server) handleRecorderRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.manager.Restart(ctx); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, map[string]any{"status": s.manager.Status()})
}

func (s *Server) handleBotRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if s.restartRequested == nil {
		s.writeError(w, http.StatusNotImplemented, errors.New("bot restart is not available"))
		return
	}

	scheduled := false
	select {
	case s.restartRequested <- struct{}{}:
		scheduled = true
	default:
	}

	s.writeJSON(w, map[string]any{
		"restarting": true,
		"scheduled":  scheduled,
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	limit := parsePositiveInt(r.URL.Query().Get("limit"), 200, 500)
	hideOffline := r.URL.Query().Get("hide_offline") == "1"
	if hideOffline {
		lines, filteredCount := applog.RecentLinesFiltered(limit, func(line string) bool {
			return !isOfflinePollingLogLine(line)
		})
		s.writeJSON(w, map[string]any{
			"lines":          lines,
			"filtered_count": filteredCount,
		})
		return
	}

	s.writeJSON(w, map[string]any{
		"lines":          applog.RecentLines(limit),
		"filtered_count": 0,
	})
}

func (s *Server) handleFileRoots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	s.writeJSON(w, s.fileRoots)
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	rootDir, targetDir, relativePath, err := s.resolvePath(r.URL.Query().Get("root"), r.URL.Query().Get("path"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	info, err := os.Stat(targetDir)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}
	if !info.IsDir() {
		s.writeError(w, http.StatusBadRequest, errors.New("target path is not a directory"))
		return
	}

	dirEntries, err := os.ReadDir(targetDir)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	sort.Slice(dirEntries, func(i, j int) bool {
		leftDir := dirEntries[i].IsDir()
		rightDir := dirEntries[j].IsDir()
		if leftDir != rightDir {
			return leftDir
		}
		return strings.ToLower(dirEntries[i].Name()) < strings.ToLower(dirEntries[j].Name())
	})

	entries := make([]FileEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		fullPath := filepath.Join(targetDir, entry.Name())
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}

		kind := "file"
		downloadable := true
		deletable := true
		if entry.Type()&os.ModeSymlink != 0 {
			kind = "symlink"
			downloadable = false
			deletable = false
		} else if entry.IsDir() {
			kind = "dir"
			downloadable = false
		}

		relEntryPath, _ := filepath.Rel(rootDir, fullPath)
		entries = append(entries, FileEntry{
			Name:         entry.Name(),
			Path:         filepath.ToSlash(relEntryPath),
			Type:         kind,
			Size:         fileInfo.Size(),
			ModifiedAt:   fileInfo.ModTime().Format(time.RFC3339),
			Downloadable: downloadable,
			Deletable:    deletable,
		})
	}

	parent := ""
	if relativePath != "" {
		parent = filepath.ToSlash(filepath.Dir(relativePath))
		if parent == "." {
			parent = ""
		}
	}

	s.writeJSON(w, FileListResponse{
		Root:    rootDir,
		Path:    filepath.ToSlash(relativePath),
		Parent:  parent,
		Entries: entries,
	})
}

func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	_, targetFile, _, err := s.resolvePath(r.URL.Query().Get("root"), r.URL.Query().Get("path"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	info, err := os.Lstat(targetFile)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		s.writeError(w, http.StatusBadRequest, errors.New("symlink download is not supported"))
		return
	}
	if info.IsDir() {
		s.writeError(w, http.StatusBadRequest, errors.New("directory download is not supported"))
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(targetFile)))
	http.ServeFile(w, r, targetFile)
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	var req struct {
		Root string `json:"root"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	_, targetPath, _, err := s.resolvePath(req.Root, req.Path)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	recordingsRoot := s.recordingsRoot
	if recordingsRoot == "" {
		recordingsRoot = filepath.Clean(filepath.Join(s.options.RootDir, "Recordings"))
		if abs, err := filepath.Abs(recordingsRoot); err == nil {
			recordingsRoot = filepath.Clean(abs)
		}
	}
	if !pathWithinRoot(recordingsRoot, targetPath) {
		s.writeError(w, http.StatusBadRequest, errors.New("deletion is limited to the Recordings directory"))
		return
	}
	if filepath.Clean(targetPath) == filepath.Clean(recordingsRoot) {
		s.writeError(w, http.StatusBadRequest, errors.New("deleting the Recordings root is not supported"))
		return
	}

	info, err := os.Lstat(targetPath)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		s.writeError(w, http.StatusBadRequest, errors.New("symlink deletion is not supported"))
		return
	}

	if info.IsDir() {
		err = os.RemoveAll(targetPath)
	} else {
		err = os.Remove(targetPath)
	}
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	s.writeJSON(w, map[string]any{"deleted": true})
}

func (s *Server) handleFileTelegramUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	var req struct {
		Root string `json:"root"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	_, targetFile, _, err := s.resolvePath(req.Root, req.Path)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	info, err := os.Lstat(targetFile)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		s.writeError(w, http.StatusBadRequest, errors.New("symlink Telegram upload is not supported"))
		return
	}
	if info.IsDir() {
		s.writeError(w, http.StatusBadRequest, errors.New("directory Telegram upload is not supported"))
		return
	}

	telegramCfg := telegram.LoadConfig()
	if !telegramCfg.Enabled {
		s.writeError(w, http.StatusBadRequest, errors.New("Telegram uploads are disabled"))
		return
	}
	if strings.TrimSpace(telegramCfg.BotToken) == "" || strings.TrimSpace(telegramCfg.ChatID) == "" {
		s.writeError(w, http.StatusBadRequest, errors.New("Telegram bot token and chat_id are required"))
		return
	}

	// Manual uploads must keep the original file; audio goes to sendAudio and everything else goes to sendDocument.
	method, err := telegram.UploadManagedFile(telegramCfg, targetFile, telegramCaption(filepath.Base(targetFile)))
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}

	s.writeJSON(w, map[string]any{
		"uploaded": true,
		"file":     filepath.Base(targetFile),
		"method":   method,
	})
}

func (s *Server) resolvePath(requestedRoot, requestedPath string) (string, string, string, error) {
	allowedRoots := s.fileRoots
	rootDir := filepath.Clean(strings.TrimSpace(requestedRoot))
	if rootDir == "" && len(allowedRoots) > 0 {
		rootDir = filepath.Clean(allowedRoots[0].Root)
	}

	rootAbs, ok := s.fileRootByPath[rootDir]
	if !ok {
		if abs, err := filepath.Abs(rootDir); err == nil {
			rootAbs = s.fileRootByPath[filepath.Clean(abs)]
		}
	}
	if rootAbs == "" {
		return "", "", "", errors.New("unknown file root")
	}

	relativePath := filepath.Clean(filepath.FromSlash(strings.TrimSpace(requestedPath)))
	if relativePath == "." {
		relativePath = ""
	}

	targetAbs := filepath.Clean(filepath.Join(rootAbs, relativePath))
	targetAbs, err := filepath.Abs(targetAbs)
	if err != nil {
		return "", "", "", err
	}

	if !pathWithinRoot(rootAbs, targetAbs) {
		return "", "", "", errors.New("path escapes the allowed root")
	}

	return rootAbs, targetAbs, relativePath, nil
}

func (s *Server) buildDiagnostics(app config.AppConfig, telegramCfg telegram.Config, status service.Status, ffmpegPath string) []Diagnostic {
	diagnostics := []Diagnostic{}
	add := func(level, message string) {
		diagnostics = append(diagnostics, Diagnostic{Level: level, Message: message})
	}

	if config.EnabledStreamers(&app) == 0 {
		add("warn", "No enabled streamers are configured. The recorder cannot start until at least one streamer is enabled.")
	}
	if telegramCfg.Enabled && telegramCfg.ConvertToM4A && ffmpegPath == "" {
		add("error", "Telegram audio extraction is enabled, but ffmpeg is not available in PATH.")
	}
	if telegramCfg.Enabled && strings.Contains(strings.TrimSpace(telegramCfg.ApiEndpoint), "127.0.0.1:8081") {
		add("warn", "Telegram uploads are pointed at a local Bot API endpoint. Make sure that service also runs on the VPS, or switch back to https://api.telegram.org.")
	}
	if IsPublicListen(s.options.Address) && s.options.Username == "" {
		add("error", "The web console is exposed on a non-loopback address without built-in authentication.")
	}
	if runtime.GOOS != "linux" {
		add("info", "This code is currently running on a non-Linux machine. A final smoke test on Ubuntu 24.04 LTS is still recommended.")
	}
	if status.Running && status.ScheduledJobs == 0 {
		add("warn", "The recorder is running, but no schedules are currently active.")
	}
	for _, warning := range status.Warnings {
		switch warning.Code {
		case "stream_password_required":
			message := fmt.Sprintf("Streamer [%s] is currently locked by a TwitCasting password. Recording stays paused until you fill in the password field in General & Streamer Settings.", warning.Streamer)
			if !warning.RetryAt.IsZero() {
				message += fmt.Sprintf(" Next automatic check: %s.", warning.RetryAt.Local().Format("2006-01-02 15:04:05"))
			}
			add("warn", message)

		case "streamer_id_invalid":
			message := fmt.Sprintf("Streamer [%s] 的 ID已失效。請到 General & Streamer Settings 檢查並更新 screen-id。", warning.Streamer)
			add("warn", message)

		case "stream_member_only":
			message := fmt.Sprintf("Streamer [%s] 目前是會員限定直播。Discord 仍會通知，但系統不會嘗試錄製，因此不會顯示在進行中的錄影列表。", warning.Streamer)
			add("info", message)
		}
	}
	return diagnostics
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	if s.options.Username == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(username), []byte(s.options.Username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(password), []byte(s.options.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Twitcast Admin"`)
			s.writeError(w, http.StatusUnauthorized, errors.New("authentication required"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) assetHandler() http.Handler {
	assetFS, err := fs.Sub(assets, "assets")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.StripPrefix("/assets/", http.FileServer(http.FS(assetFS)))
}

func (s *Server) writeJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func (s *Server) writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	s.writeJSON(w, map[string]string{"error": err.Error()})
}

func (s *Server) methodNotAllowed(w http.ResponseWriter) {
	s.writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func parsePositiveInt(raw string, fallback int, max int) int {
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func pathWithinRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)))
}

func telegramCaption(name string) string {
	caption := strings.TrimSpace(name)
	if caption == "" {
		return "manual upload"
	}
	runes := []rune(caption)
	if len(runes) <= 900 {
		return caption
	}
	return string(runes[:900]) + "..."
}

func isOfflinePollingLogLine(line string) bool {
	return strings.Contains(line, "Error fetching stream URL for streamer [") &&
		strings.Contains(line, "live stream is offline")
}

// IsPublicListen checks whether the bind address is publicly reachable and should be treated as a security-sensitive bind target.
// 解析方式與 web 入口一致：若未指定位址視為公共位址；僅 127.0.0.1 / localhost / ::1 視為私有可直接回收。
func IsPublicListen(addr string) bool {
	host := strings.TrimSpace(addr)
	if parsedHost, _, err := net.SplitHostPort(addr); err == nil {
		host = parsedHost
	}

	switch host {
	case "", "0.0.0.0", "::":
		return true
	case "127.0.0.1", "localhost", "::1":
		return false
	default:
		return true
	}
}
