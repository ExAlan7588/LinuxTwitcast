package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
	"github.com/jzhang046/croned-twitcasting-recorder/twitcasting"
)

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

	target, err := archiveSessionRecordingPath(refreshed, original.Filename)
	if err != nil {
		log.Printf("[Metadata] Failed building archive target for %s: %v\n", original.Filename, err)
		return refreshed
	}
	if target == "" || target == original.Filename {
		return refreshed
	}

	if err := moveFileWithoutOverwrite(original.Filename, target); err != nil {
		log.Printf("[Metadata] Failed renaming recording file %s to %s: %v\n", original.Filename, target, err)
		return refreshed
	}

	log.Printf("[Metadata] Renamed recording file %s to %s\n", original.Filename, target)
	refreshed.Filename = target
	return refreshed
}

func archiveSessionRecordingPath(session record.SessionInfo, currentPath string) (string, error) {
	dir := filepath.Dir(currentPath)
	ext := filepath.Ext(currentPath)
	if ext == "" {
		ext = ".ts"
	}

	fileName := record.FormattedMediaName(session) + ext
	target := filepath.Join(dir, fileName)
	available, err := nextAvailablePath(target)
	if err != nil {
		return "", err
	}
	return available, nil
}

func nextAvailablePath(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, nil
	} else if err != nil {
		return "", err
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, index, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}

func moveFileWithoutOverwrite(sourcePath, targetPath string) error {
	if err := os.Link(sourcePath, targetPath); err != nil {
		return err
	}
	if err := os.Remove(sourcePath); err != nil {
		_ = os.Remove(targetPath)
		return err
	}
	return nil
}
