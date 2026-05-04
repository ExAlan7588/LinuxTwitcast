package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
	"github.com/jzhang046/croned-twitcasting-recorder/twitcasting"
)

const recordingDateLayout = "2006-01-02"

var lookupMovieArchiveMetadata = twitcasting.LookupMovieArchiveMetadata

func refreshSessionFromArchiveMetadata(session record.SessionInfo) record.SessionInfo {
	streamer := strings.TrimSpace(session.Streamer)
	movieID := strings.TrimSpace(session.MovieID)
	if streamer == "" || movieID == "" {
		return session
	}

	metadata, err := lookupMovieArchiveMetadata(streamer, movieID)
	if err != nil {
		log.Printf("[Metadata] Failed refreshing archive metadata for [%s] movie [%s]: %v\n", streamer, movieID, err)
		return session
	}

	refreshed := applyArchiveMetadata(session, metadata)
	return renameSessionRecordingFile(session, refreshed)
}

func applyArchiveMetadata(session record.SessionInfo, metadata twitcasting.MovieArchiveMetadata) record.SessionInfo {
	refreshed := session
	if streamerName := strings.TrimSpace(metadata.StreamerName); streamerName != "" {
		refreshed.StreamerName = streamerName
	}
	if title := strings.TrimSpace(metadata.Title); title != "" {
		refreshed.Title = title
	}
	if avatarURL := strings.TrimSpace(metadata.AvatarURL); avatarURL != "" {
		refreshed.AvatarURL = avatarURL
	}
	if coverURL := strings.TrimSpace(metadata.CoverURL); coverURL != "" {
		refreshed.CoverURL = coverURL
	}
	if metadata.MemberOnly {
		refreshed.MemberOnly = true
	}
	return refreshed
}

func renameSessionRecordingFile(original, refreshed record.SessionInfo) record.SessionInfo {
	if strings.TrimSpace(original.Filename) == "" || strings.TrimSpace(refreshed.Title) == "" {
		return refreshed
	}

	target := archiveSessionRecordingPath(refreshed, original.Filename)
	if target == "" || target == original.Filename {
		return refreshed
	}
	if err := os.Rename(original.Filename, target); err != nil {
		log.Printf("[Metadata] Failed renaming recording file %s to %s: %v\n", original.Filename, target, err)
		return refreshed
	}

	log.Printf("[Metadata] Renamed recording file %s to %s\n", original.Filename, target)
	refreshed.Filename = target
	return refreshed
}

func archiveSessionRecordingPath(session record.SessionInfo, currentPath string) string {
	dir := filepath.Dir(currentPath)
	ext := filepath.Ext(currentPath)
	if ext == "" {
		ext = ".ts"
	}

	name := firstNonEmpty(session.StreamerName, session.Streamer)
	title := strings.TrimSpace(session.Title)
	if name == "" || title == "" {
		return ""
	}

	fileName := fmt.Sprintf("[%s][%s]%s%s", name, sessionRecordingDate(session), title, ext)
	if dir == "." || dir == "" {
		return fileName
	}
	return filepath.Join(dir, fileName)
}

func sessionRecordingDate(session record.SessionInfo) string {
	if session.StartedAt.IsZero() {
		return time.Now().Format(recordingDateLayout)
	}
	return session.StartedAt.Local().Format(recordingDateLayout)
}
