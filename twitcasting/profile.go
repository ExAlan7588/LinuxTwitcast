package twitcasting

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/config"
)

const (
	apiV2BaseURL               = "https://apiv2.twitcasting.tv"
	apiV2Version               = "2.0"
	profileAPICacheTTL         = 30 * time.Second
	twitCastingClientIDEnv     = "TWITCASTING_CLIENT_ID"
	twitCastingClientSecretEnv = "TWITCASTING_CLIENT_SECRET"
)

type twitCastingProfileAPICredentials struct {
	ClientID     string
	ClientSecret string
}

type twitCastingUserResponse struct {
	User twitCastingUser `json:"user"`
}

type twitCastingUser struct {
	ID       string `json:"id"`
	ScreenID string `json:"screen_id"`
	Name     string `json:"name"`
	Image    string `json:"image"`
}

var profileAPICache struct {
	mu       sync.RWMutex
	loadedAt time.Time
	creds    twitCastingProfileAPICredentials
}

func enrichStreamInfoFromUserAPI(streamer string, info *streamPageInfo) {
	if info == nil {
		return
	}

	user, err := fetchUserProfileFromAPI(streamer)
	if err != nil {
		return
	}

	if name := strings.TrimSpace(user.Name); name != "" {
		info.streamerName = sanitizeFilename(name)
	}
	if avatarURL := normalizeImageURL(user.Image); avatarURL != "" {
		info.avatarURL = avatarURL
	}
}

func fetchUserProfileFromAPI(streamer string) (twitCastingUser, error) {
	creds := currentProfileAPICredentials()
	if !creds.IsComplete() {
		return twitCastingUser{}, errors.New("TwitCasting API credentials are not configured")
	}

	endpoint := fmt.Sprintf("%s/users/%s", apiV2BaseURL, url.PathEscape(strings.TrimSpace(streamer)))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return twitCastingUser{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", creds.BasicAuthHeader())
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Api-Version", apiV2Version)

	resp, err := httpClient.Do(req)
	if err != nil {
		return twitCastingUser{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return twitCastingUser{}, ErrStreamerNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return twitCastingUser{}, fmt.Errorf("TwitCasting API request failed: status %d", resp.StatusCode)
	}

	var payload twitCastingUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return twitCastingUser{}, err
	}
	if strings.TrimSpace(payload.User.ScreenID) == "" &&
		strings.TrimSpace(payload.User.Name) == "" &&
		strings.TrimSpace(payload.User.Image) == "" {
		return twitCastingUser{}, errors.New("TwitCasting API response did not include user data")
	}
	return payload.User, nil
}

func (c twitCastingProfileAPICredentials) IsComplete() bool {
	return strings.TrimSpace(c.ClientID) != "" && strings.TrimSpace(c.ClientSecret) != ""
}

func (c twitCastingProfileAPICredentials) BasicAuthHeader() string {
	token := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(c.ClientID) + ":" + strings.TrimSpace(c.ClientSecret)))
	return "Basic " + token
}

func currentProfileAPICredentials() twitCastingProfileAPICredentials {
	if creds := profileAPICredentialsFromEnv(); creds.IsComplete() {
		return creds
	}

	profileAPICache.mu.RLock()
	if time.Since(profileAPICache.loadedAt) < profileAPICacheTTL {
		cached := profileAPICache.creds
		profileAPICache.mu.RUnlock()
		return cached
	}
	profileAPICache.mu.RUnlock()

	profileAPICache.mu.Lock()
	defer profileAPICache.mu.Unlock()
	if time.Since(profileAPICache.loadedAt) < profileAPICacheTTL {
		return profileAPICache.creds
	}

	loaded := twitCastingProfileAPICredentials{}
	cfg, err := config.LoadDefault()
	if err == nil && cfg != nil {
		loaded = twitCastingProfileAPICredentials{
			ClientID:     strings.TrimSpace(cfg.TwitCastingAPI.ClientID),
			ClientSecret: strings.TrimSpace(cfg.TwitCastingAPI.ClientSecret),
		}
	}
	profileAPICache.loadedAt = time.Now()
	profileAPICache.creds = loaded
	return loaded
}

func profileAPICredentialsFromEnv() twitCastingProfileAPICredentials {
	return twitCastingProfileAPICredentials{
		ClientID:     strings.TrimSpace(os.Getenv(twitCastingClientIDEnv)),
		ClientSecret: strings.TrimSpace(os.Getenv(twitCastingClientSecretEnv)),
	}
}

func InvalidateProfileAPICache() {
	profileAPICache.mu.Lock()
	defer profileAPICache.mu.Unlock()
	profileAPICache.loadedAt = time.Time{}
	profileAPICache.creds = twitCastingProfileAPICredentials{}
}
