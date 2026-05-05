package record

import (
	"fmt"
	"strings"
	"time"
)

const mediaDateLayout = "2006-01-02"

func NormalizedStreamerLabel(session SessionInfo) string {
	if label := strings.TrimSpace(session.StreamerName); label != "" {
		return label
	}
	return strings.TrimSpace(session.Streamer)
}

func NormalizedSessionDate(session SessionInfo) string {
	if session.StartedAt.IsZero() {
		return time.Now().Format(mediaDateLayout)
	}
	return session.StartedAt.Local().Format(mediaDateLayout)
}

func FormattedMediaName(session SessionInfo) string {
	streamer := NormalizedStreamerLabel(session)
	date := NormalizedSessionDate(session)
	title := strings.TrimSpace(session.Title)

	if title == "" {
		return fmt.Sprintf("[%s][%s]", streamer, date)
	}
	return fmt.Sprintf("[%s][%s]%s", streamer, date, title)
}
