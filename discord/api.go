package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// apiCall makes an authenticated Discord API request.
// It is a package-level helper shared by Notifier, commands, and the gateway.
func apiCall(botToken, method, endpoint string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, discordAPI+endpoint, reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bot "+botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, nil
}

func SendTestMessage(cfg Config, content string) error {
	channelID := strings.TrimSpace(cfg.NotifyChannelID)
	if strings.TrimSpace(cfg.BotToken) == "" {
		return fmt.Errorf("discord bot token is required")
	}
	if channelID == "" {
		return fmt.Errorf("discord notify channel ID is required")
	}

	respBody, status, err := apiCall(cfg.BotToken, http.MethodPost, "/channels/"+channelID+"/messages", map[string]string{
		"content": content,
	})
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("discord API error %d: %s", status, string(respBody))
	}
	return nil
}
