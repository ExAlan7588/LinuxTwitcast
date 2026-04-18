package twitcasting

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

const authConfigPath = "twitcasting_auth.json"

type AuthConfig struct {
	CookieHeader string `json:"cookie_header,omitempty"`
}

type AuthStatus struct {
	Configured  bool `json:"configured"`
	CookieCount int  `json:"cookie_count"`
}

var authCache struct {
	mu     sync.RWMutex
	loaded bool
	cfg    AuthConfig
}

func LoadAuthConfig() AuthConfig {
	authCache.mu.RLock()
	if authCache.loaded {
		cfg := authCache.cfg
		authCache.mu.RUnlock()
		return cfg
	}
	authCache.mu.RUnlock()

	authCache.mu.Lock()
	defer authCache.mu.Unlock()
	if authCache.loaded {
		return authCache.cfg
	}

	cfg, err := loadAuthConfigFromDisk()
	if err != nil {
		authCache.loaded = true
		authCache.cfg = AuthConfig{}
		return AuthConfig{}
	}

	authCache.loaded = true
	authCache.cfg = cfg
	return cfg
}

func SaveAuthConfig(cfg AuthConfig) error {
	normalized, err := normalizeAuthConfig(cfg)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(normalized, "", "    ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(authConfigPath, data, 0600); err != nil {
		return err
	}

	InvalidateAuthCache()
	return nil
}

func ClearAuthConfig() error {
	if err := os.Remove(authConfigPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	InvalidateAuthCache()
	return nil
}

func InvalidateAuthCache() {
	authCache.mu.Lock()
	defer authCache.mu.Unlock()
	authCache.loaded = false
	authCache.cfg = AuthConfig{}
}

func CurrentAuthStatus() AuthStatus {
	cfg := LoadAuthConfig()
	cookieHeader := strings.TrimSpace(cfg.CookieHeader)
	return AuthStatus{
		Configured:  cookieHeader != "",
		CookieCount: countCookiePairs(cookieHeader),
	}
}

func CurrentCookieHeader() string {
	return strings.TrimSpace(LoadAuthConfig().CookieHeader)
}

// 浏览器导出的 cookie 可能是 Cookie header、纯 name=value 列表，或 Netscape cookie 文件。
func NormalizeCookieInput(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("cookie content is required")
	}

	if header, ok := parseCookieHeader(trimmed); ok {
		return header, nil
	}
	if header, ok := parseNetscapeCookieFile(trimmed); ok {
		return header, nil
	}

	return "", errors.New("unsupported cookie format; upload a Cookie header or Netscape cookie file")
}

func ApplyAuthToRequest(req *http.Request) {
	if req == nil || req.URL == nil {
		return
	}
	if !strings.HasSuffix(strings.ToLower(req.URL.Hostname()), "twitcasting.tv") {
		return
	}
	applyCookieHeader(req.Header)
}

func ApplyAuthToHeaders(headers http.Header) {
	applyCookieHeader(headers)
}

func loadAuthConfigFromDisk() (AuthConfig, error) {
	data, err := os.ReadFile(authConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return AuthConfig{}, nil
		}
		return AuthConfig{}, err
	}

	var cfg AuthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AuthConfig{}, err
	}
	return normalizeAuthConfig(cfg)
}

func normalizeAuthConfig(cfg AuthConfig) (AuthConfig, error) {
	cookieHeader := strings.TrimSpace(cfg.CookieHeader)
	if cookieHeader == "" {
		return AuthConfig{}, nil
	}

	normalized, err := NormalizeCookieInput(cookieHeader)
	if err != nil {
		return AuthConfig{}, err
	}
	return AuthConfig{CookieHeader: normalized}, nil
}

func applyCookieHeader(headers http.Header) {
	if headers == nil || strings.TrimSpace(headers.Get("Cookie")) != "" {
		return
	}
	cookieHeader := CurrentCookieHeader()
	if cookieHeader == "" {
		return
	}
	headers.Set("Cookie", cookieHeader)
}

func parseCookieHeader(raw string) (string, bool) {
	candidate := strings.TrimSpace(raw)
	candidate = strings.TrimPrefix(candidate, "Cookie:")
	candidate = strings.TrimPrefix(candidate, "cookie:")
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return "", false
	}
	if strings.Contains(candidate, "\t") || strings.Contains(candidate, "\n") || strings.Contains(candidate, "\r") {
		return "", false
	}

	parts := strings.Split(candidate, ";")
	cookies := make([]string, 0, len(parts))
	seen := make(map[string]int, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		name, value, ok := strings.Cut(token, "=")
		if !ok {
			return "", false
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" {
			return "", false
		}
		pair := fmt.Sprintf("%s=%s", name, value)
		if idx, exists := seen[name]; exists {
			cookies[idx] = pair
			continue
		}
		seen[name] = len(cookies)
		cookies = append(cookies, pair)
	}
	if len(cookies) == 0 {
		return "", false
	}
	return strings.Join(cookies, "; "), true
}

func parseNetscapeCookieFile(raw string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	cookies := make([]string, 0, len(lines))
	seen := make(map[string]int, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "#HttpOnly_") {
			continue
		}

		fields := strings.Split(trimmed, "\t")
		if len(fields) < 7 {
			return "", false
		}

		domain := strings.TrimPrefix(strings.TrimSpace(fields[0]), "#HttpOnly_")
		if !strings.HasSuffix(strings.ToLower(domain), "twitcasting.tv") {
			continue
		}

		name := strings.TrimSpace(fields[5])
		value := strings.TrimSpace(fields[6])
		if name == "" {
			continue
		}
		pair := fmt.Sprintf("%s=%s", name, value)
		if idx, exists := seen[name]; exists {
			cookies[idx] = pair
			continue
		}
		seen[name] = len(cookies)
		cookies = append(cookies, pair)
	}
	if len(cookies) == 0 {
		return "", false
	}
	return strings.Join(cookies, "; "), true
}

func countCookiePairs(cookieHeader string) int {
	if strings.TrimSpace(cookieHeader) == "" {
		return 0
	}
	count := 0
	for _, part := range strings.Split(cookieHeader, ";") {
		if strings.TrimSpace(part) != "" {
			count++
		}
	}
	return count
}
