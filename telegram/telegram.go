package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
)

type Config struct {
	Enabled      bool   `json:"enabled"`
	BotToken     string `json:"bot_token"`
	ChatID       string `json:"chat_id"`
	ApiEndpoint  string `json:"api_endpoint"`
	ConvertToM4A bool   `json:"convert_to_m4a"`
	KeepOriginal bool   `json:"keep_original"`
}

type UploadMethod string

type UploadResult struct {
	Method     UploadMethod
	MessageURL string
}

type m4aStrategy struct {
	Name string
	Args []string
}

type telegramAPIResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      telegramMessage `json:"result"`
}

type telegramMessage struct {
	MessageID int64        `json:"message_id"`
	Chat      telegramChat `json:"chat"`
}

type telegramChat struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

const (
	UploadMethodAudio    UploadMethod = "audio"
	UploadMethodDocument UploadMethod = "document"
)

var (
	runFFmpeg = func(args ...string) ([]byte, error) {
		return exec.Command("ffmpeg", args...).CombinedOutput()
	}
	uploadTelegramFile = UploadFileWithResult
)

func LoadConfig() Config {
	c := Config{
		ApiEndpoint: "https://api.telegram.org",
	}
	data, err := os.ReadFile("telegram.json")
	if err == nil {
		json.Unmarshal(data, &c)
	}
	if c.ApiEndpoint == "" {
		c.ApiEndpoint = "https://api.telegram.org"
	}
	return c
}

func SaveConfig(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile("telegram.json", data, 0600)
}

// Process handles post-recording task. Runs synchronously so run in a goroutine.
func Process(cfg Config, session record.SessionInfo) UploadResult {
	if !cfg.Enabled || cfg.BotToken == "" || cfg.ChatID == "" {
		return UploadResult{}
	}

	targetFile := session.Filename
	// 1. Convert to M4A if needed
	if cfg.ConvertToM4A {
		m4aFile := taggedM4APath(session)
		log.Printf("[Telegram] Extracting audio to %s\n", m4aFile)
		err := ConvertFileToM4A(session, targetFile, m4aFile)
		if err != nil {
			log.Printf("[Telegram] FFmpeg extraction failed: %v", err)
			return UploadResult{}
		} else {
			log.Printf("[Telegram] FFmpeg extraction successful")
			targetFile = m4aFile
			if !cfg.KeepOriginal {
				os.Remove(session.Filename)
				log.Printf("[Telegram] Removed original file %s", session.Filename)
			}
		}
	}

	// 2. Upload to Telegram
	log.Printf("[Telegram] Uploading %s to Telegram Chat %s", targetFile, cfg.ChatID)
	result, err := uploadTelegramFile(cfg, targetFile, telegramCaption(session))
	if err != nil {
		log.Printf("[Telegram] Upload failed: %v\n", err)
		return UploadResult{}
	} else {
		log.Printf("[Telegram] Upload successful: %s\n", targetFile)
	}
	return result
}

func ConvertManagedMediaFile(session record.SessionInfo, filePath string) (string, error) {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".ts", ".mp4":
	default:
		return "", fmt.Errorf("only .ts or .mp4 files can be converted to m4a")
	}

	outputFile := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".m4a"
	session = normalizeManagedConversionSession(session, filePath)

	if err := ConvertFileToM4A(session, filePath, outputFile); err != nil {
		return "", err
	}
	return outputFile, nil
}

func normalizeManagedConversionSession(session record.SessionInfo, filePath string) record.SessionInfo {
	if strings.TrimSpace(session.Filename) == "" {
		session.Filename = filePath
	}
	if strings.TrimSpace(session.Title) == "" {
		session.Title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}
	if session.StartedAt.IsZero() {
		if info, err := os.Stat(filePath); err == nil {
			session.StartedAt = info.ModTime()
		} else {
			session.StartedAt = time.Now()
		}
	}
	return session
}

func ConvertFileToM4A(session record.SessionInfo, inputFile, outputFile string) error {
	coverFile, cleanupCover, err := downloadCoverArt(sessionCoverArtURL(session))
	if err != nil {
		log.Printf("[Telegram] Failed downloading cover art: %v", err)
	}
	if cleanupCover != nil {
		defer cleanupCover()
	}

	failures := make([]string, 0, 4)
	for _, strategy := range buildM4AStrategies(session, inputFile, outputFile, coverFile) {
		out, err := runFFmpeg(strategy.Args...)
		if err == nil {
			return nil
		}
		_ = os.Remove(outputFile)
		failures = append(failures, fmt.Sprintf("%s: %v | %s", strategy.Name, err, strings.TrimSpace(string(out))))
	}

	return fmt.Errorf("all m4a conversion attempts failed: %s", strings.Join(failures, " || "))
}

func buildM4AStrategies(session record.SessionInfo, inputFile, outputFile, coverFile string) []m4aStrategy {
	return []m4aStrategy{
		{
			Name: "copy-audio",
			Args: buildM4AArgs(session, inputFile, outputFile, coverFile, false, false),
		},
		{
			Name: "force-mpegts-copy-audio",
			Args: buildM4AArgs(session, inputFile, outputFile, coverFile, true, false),
		},
		{
			Name: "reencode-aac",
			Args: buildM4AArgs(session, inputFile, outputFile, coverFile, false, true),
		},
		{
			Name: "force-mpegts-reencode-aac",
			Args: buildM4AArgs(session, inputFile, outputFile, coverFile, true, true),
		},
	}
}

// 这里把文件名、元数据和封面统一在一个地方生成，避免 ffmpeg 参数散落在流程里难维护。
func buildM4AArgs(session record.SessionInfo, inputFile, outputFile, coverFile string, forceMpegTS bool, reencodeAAC bool) []string {
	args := []string{"-y"}
	if forceMpegTS {
		args = append(args, "-f", "mpegts")
	}
	args = append(args, "-i", inputFile)

	audioArgs := []string{"-c:a", "copy"}
	if reencodeAAC {
		audioArgs = []string{"-c:a", "aac", "-b:a", "192k"}
	}

	if strings.TrimSpace(coverFile) != "" {
		args = append(args, "-i", coverFile, "-map", "0:a:0", "-map", "1:v:0")
		args = append(args, audioArgs...)
		args = append(args, "-c:v", "mjpeg", "-disposition:v:0", "attached_pic")
	} else {
		args = append(args, "-vn")
		args = append(args, audioArgs...)
	}
	args = append(args, buildM4AMetadataArgs(session)...)
	if strings.TrimSpace(coverFile) != "" {
		args = append(args, "-metadata:s:v", "title=Cover", "-metadata:s:v", "comment=Cover (front)")
	}
	args = append(args, outputFile)
	return args
}

func buildM4AMetadataArgs(session record.SessionInfo) []string {
	metadata := []struct {
		key   string
		value string
	}{
		{key: "artist", value: telegramArtist(session)},
		{key: "title", value: strings.TrimSpace(session.Title)},
		{key: "date", value: sessionDate(session).Format("2006-01-02")},
	}

	args := make([]string, 0, len(metadata)*2)
	for _, item := range metadata {
		if item.value == "" {
			continue
		}
		args = append(args, "-metadata", fmt.Sprintf("%s=%s", item.key, item.value))
	}
	return args
}

func taggedM4APath(session record.SessionInfo) string {
	dir := filepath.Dir(session.Filename)
	name := fmt.Sprintf("[%s][%s]", telegramArtist(session), sessionDate(session).Format("2006-01-02"))
	if title := strings.TrimSpace(session.Title); title != "" {
		name += title
	}
	return filepath.Join(dir, name+".m4a")
}

func telegramCaption(session record.SessionInfo) string {
	artist := telegramArtist(session)
	title := strings.TrimSpace(session.Title)
	if title == "" {
		return fmt.Sprintf("[%s]", artist)
	}
	return fmt.Sprintf("[%s] %s", artist, title)
}

func telegramArtist(session record.SessionInfo) string {
	if artist := strings.TrimSpace(session.StreamerName); artist != "" {
		return artist
	}
	return strings.TrimSpace(session.Streamer)
}

func sessionDate(session record.SessionInfo) time.Time {
	if session.StartedAt.IsZero() {
		return time.Now()
	}
	return session.StartedAt.Local()
}

func sessionCoverArtURL(session record.SessionInfo) string {
	if avatarURL := strings.TrimSpace(session.AvatarURL); avatarURL != "" {
		return avatarURL
	}
	return strings.TrimSpace(session.CoverURL)
}

// 封面下载失败不应阻塞上传；这里尽量拿到临时图片给 ffmpeg 写 attached_pic，失败就回退成纯音频标签。
func downloadCoverArt(rawURL string) (string, func(), error) {
	coverURL := strings.TrimSpace(rawURL)
	if coverURL == "" {
		return "", nil, nil
	}

	req, err := http.NewRequest(http.MethodGet, coverURL, nil)
	if err != nil {
		return "", nil, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("bad status %d", resp.StatusCode)
	}

	ext := filepath.Ext(strings.TrimSpace(resp.Request.URL.Path))
	if ext == "" {
		ext = ".jpg"
	}
	tempFile, err := os.CreateTemp("", "linuxtwitcast-cover-*"+ext)
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", nil, err
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return "", nil, err
	}
	return tempFile.Name(), func() { _ = os.Remove(tempFile.Name()) }, nil
}

func UploadFile(cfg Config, filePath string, caption string) error {
	_, err := UploadFileWithResult(cfg, filePath, caption)
	return err
}

func UploadManagedFile(cfg Config, filePath string, caption string) (UploadMethod, error) {
	method := uploadMethodForPath(filePath)
	result, err := uploadFile(cfg, filePath, caption, method)
	return result.Method, err
}

func UploadFileWithResult(cfg Config, filePath string, caption string) (UploadResult, error) {
	return uploadFile(cfg, filePath, caption, UploadMethodAudio)
}

func SendTestMessage(cfg Config, text string) error {
	if strings.TrimSpace(cfg.BotToken) == "" {
		return fmt.Errorf("telegram bot token is required")
	}
	if strings.TrimSpace(cfg.ChatID) == "" {
		return fmt.Errorf("telegram chat_id is required")
	}

	form := url.Values{}
	form.Set("chat_id", cfg.ChatID)
	form.Set("text", text)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(apiEndpoint(cfg.ApiEndpoint), "/"), cfg.BotToken), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bad status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func uploadMethodForPath(filePath string) UploadMethod {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".aac", ".flac", ".m4a", ".m4b", ".mp3", ".oga", ".ogg", ".opus", ".wav", ".wma":
		return UploadMethodAudio
	default:
		return UploadMethodDocument
	}
}

func uploadFile(cfg Config, filePath string, caption string, method UploadMethod) (UploadResult, error) {
	result := UploadResult{Method: method}

	file, err := os.Open(filePath)
	if err != nil {
		return result, err
	}
	defer file.Close()

	// If streaming directly via io.Pipe or multipart, we can reduce memory.
	// But local Bot API might prefer typical multipart.
	// For files >200MB, storing in memory using bytes.Buffer might OOM or be heavy.
	// We'll use io.Pipe to stream multipart directly to the request without buffering into memory.
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		_ = writer.WriteField("chat_id", cfg.ChatID)
		_ = writer.WriteField("caption", caption)

		part, err := writer.CreateFormFile(uploadFieldName(method), filepath.Base(filePath))
		if err != nil {
			return
		}
		io.Copy(part, file)
	}()

	url := fmt.Sprintf("%s/bot%s/%s", strings.TrimRight(apiEndpoint(cfg.ApiEndpoint), "/"), cfg.BotToken, uploadMethodEndpoint(method))
	req, err := http.NewRequest("POST", url, pr)
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("bad status %d: %s", resp.StatusCode, string(respBody))
	}

	var payload telegramAPIResponse
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return result, nil
	}
	if !payload.OK {
		if strings.TrimSpace(payload.Description) != "" {
			return result, fmt.Errorf("telegram api error: %s", payload.Description)
		}
		return result, fmt.Errorf("telegram api returned ok=false")
	}

	result.MessageURL = telegramMessageURL(payload.Result.Chat, payload.Result.MessageID)
	return result, nil
}

func uploadFieldName(method UploadMethod) string {
	if method == UploadMethodDocument {
		return "document"
	}
	return "audio"
}

func uploadMethodEndpoint(method UploadMethod) string {
	if method == UploadMethodDocument {
		return "sendDocument"
	}
	return "sendAudio"
}

func apiEndpoint(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "https://api.telegram.org"
	}
	return raw
}

func telegramMessageURL(chat telegramChat, messageID int64) string {
	if messageID <= 0 {
		return ""
	}

	username := strings.TrimSpace(chat.Username)
	if username != "" {
		return fmt.Sprintf("https://t.me/%s/%d", username, messageID)
	}

	chatID := strconv.FormatInt(chat.ID, 10)
	if strings.HasPrefix(chatID, "-100") {
		return fmt.Sprintf("https://t.me/c/%s/%d", strings.TrimPrefix(chatID, "-100"), messageID)
	}

	return ""
}
