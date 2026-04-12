package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	Enabled      bool   `json:"enabled"`
	BotToken     string `json:"bot_token"`
	ChatID       string `json:"chat_id"`
	ApiEndpoint  string `json:"api_endpoint"`
	ConvertToM4A bool   `json:"convert_to_m4a"`
	KeepOriginal bool   `json:"keep_original"`
}

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
func Process(cfg Config, filename, streamerName, title string) {
	if !cfg.Enabled || cfg.BotToken == "" || cfg.ChatID == "" {
		return
	}

	targetFile := filename
	// 1. Convert to M4A if needed
	if cfg.ConvertToM4A {
		ext := filepath.Ext(filename)
		base := filename[:len(filename)-len(ext)]
		m4aFile := base + ".m4a"

		log.Printf("[Telegram] Extracting audio to %s\n", m4aFile)
		cmd := exec.Command("ffmpeg", "-y", "-i", filename, "-vn", "-c:a", "copy", m4aFile)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[Telegram] FFmpeg extraction failed: %v\nOutput: %s", err, string(out))
			// Proceed with original file if ffmpeg fails
		} else {
			log.Printf("[Telegram] FFmpeg extraction successful")
			targetFile = m4aFile
			if !cfg.KeepOriginal {
				os.Remove(filename)
				log.Printf("[Telegram] Removed original file %s", filename)
			}
		}
	}

	// 2. Upload to Telegram
	log.Printf("[Telegram] Uploading %s to Telegram Chat %s", targetFile, cfg.ChatID)
	err := UploadFile(cfg, targetFile, fmt.Sprintf("[%s] %s", streamerName, title))
	if err != nil {
		log.Printf("[Telegram] Upload failed: %v\n", err)
	} else {
		log.Printf("[Telegram] Upload successful: %s\n", targetFile)
	}
}

func UploadFile(cfg Config, filePath string, caption string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
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

		// Create form file
		part, err := writer.CreateFormFile("audio", filepath.Base(filePath))
		if err != nil {
			return
		}
		io.Copy(part, file)
	}()

	url := fmt.Sprintf("%s/bot%s/sendAudio", strings.TrimRight(cfg.ApiEndpoint, "/"), cfg.BotToken)
	req, err := http.NewRequest("POST", url, pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

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
