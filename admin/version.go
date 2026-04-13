package admin

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const defaultVersion = "null"

type BuildInfo struct {
	Version       string
	Commit        string
	ShortCommit   string
	RepositoryURL string
}

type VersionCheckResponse struct {
	Version         string `json:"version"`
	CurrentCommit   string `json:"current_commit,omitempty"`
	LatestCommit    string `json:"latest_commit,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	RepoURL         string `json:"repo_url,omitempty"`
	Message         string `json:"message"`
}

func LoadBuildInfo(rootDir string) BuildInfo {
	info := BuildInfo{Version: defaultVersion}

	commit, err := gitOutput(rootDir, "rev-parse", "HEAD")
	if err == nil {
		info.Commit = commit
		info.ShortCommit = shortenCommit(commit)
	}

	remoteURL, err := gitOutput(rootDir, "remote", "get-url", "origin")
	if err == nil {
		info.RepositoryURL = normalizeRepositoryURL(remoteURL)
	}

	return info
}

func CheckForUpdates(rootDir string, buildInfo BuildInfo) VersionCheckResponse {
	response := VersionCheckResponse{
		Version:       buildInfo.Version,
		CurrentCommit: buildInfo.ShortCommit,
		RepoURL:       buildInfo.RepositoryURL,
	}

	if buildInfo.Commit == "" {
		response.Message = "Git metadata is unavailable in the current working tree."
		return response
	}

	remoteCommit, err := gitOutput(rootDir, "ls-remote", "origin", "refs/heads/main")
	if err != nil {
		response.Message = fmt.Sprintf("Could not check origin/main: %v", err)
		return response
	}

	latestCommit := parseRemoteRef(remoteCommit)
	if latestCommit == "" {
		response.Message = "origin/main returned an unexpected response."
		return response
	}

	response.LatestCommit = shortenCommit(latestCommit)
	if latestCommit == buildInfo.Commit {
		response.Message = "This build already matches origin/main."
		return response
	}

	response.UpdateAvailable = true
	if response.RepoURL != "" {
		response.Message = fmt.Sprintf("A newer build is available on origin/main (%s -> %s).", response.CurrentCommit, response.LatestCommit)
		return response
	}

	response.Message = fmt.Sprintf("A newer build is available on origin/main (%s -> %s), but no browser repository URL is configured.", response.CurrentCommit, response.LatestCommit)
	return response
}

func gitOutput(rootDir string, args ...string) (string, error) {
	targetDir := strings.TrimSpace(rootDir)
	if targetDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		targetDir = cwd
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	commandArgs := append([]string{"-C", targetDir}, args...)
	cmd := exec.CommandContext(ctx, "git", commandArgs...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", errors.New("git command timed out")
	}

	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return "", errors.New(text)
	}
	if text == "" {
		return "", errors.New("git returned empty output")
	}

	return text, nil
}

func parseRemoteRef(output string) string {
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func shortenCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if len(commit) <= 12 {
		return commit
	}
	return commit[:12]
}

func normalizeRepositoryURL(raw string) string {
	repoURL := strings.TrimSpace(raw)
	repoURL = strings.TrimSuffix(repoURL, ".git")
	repoURL = strings.TrimSuffix(repoURL, "/")
	if repoURL == "" {
		return ""
	}

	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(repoURL, "git@"), ":", 2)
		if len(parts) == 2 {
			return "https://" + parts[0] + "/" + strings.TrimPrefix(parts[1], "/")
		}
	}

	if strings.HasPrefix(repoURL, "ssh://") {
		parsed, err := url.Parse(repoURL)
		if err == nil && parsed.Host != "" && parsed.Path != "" {
			return "https://" + parsed.Host + "/" + strings.TrimPrefix(parsed.Path, "/")
		}
	}

	if strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://") {
		parsed, err := url.Parse(repoURL)
		if err == nil {
			parsed.Scheme = "https"
			parsed.User = nil
			parsed.RawQuery = ""
			parsed.Fragment = ""
			parsed.Path = strings.TrimSuffix(parsed.Path, ".git")
			return strings.TrimSuffix(parsed.String(), "/")
		}
	}

	return repoURL
}
