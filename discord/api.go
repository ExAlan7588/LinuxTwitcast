package discord

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
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
