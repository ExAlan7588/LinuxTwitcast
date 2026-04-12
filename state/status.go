package state

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type StreamerState struct {
	Status    string    `json:"status"` // "recording" or "processing"
	StartedAt time.Time `json:"started_at"`
}

var (
	stateMap = make(map[string]StreamerState)
	mu       sync.Mutex
	filename = "state.json"
)

// Update sets the status for a streamer and writes to state.json
func Update(screenID, status string) {
	mu.Lock()
	defer mu.Unlock()
	
	// If it already exists, maintain its original start time
	// If it doesn't, this means it's a new recording session
	if s, exists := stateMap[screenID]; exists {
		s.Status = status
		stateMap[screenID] = s
	} else {
		stateMap[screenID] = StreamerState{
			Status:    status,
			StartedAt: time.Now(),
		}
	}
	save()
}

// Clear removes a streamer from state.json when it goes standby
func Clear(screenID string) {
	mu.Lock()
	defer mu.Unlock()
	
	delete(stateMap, screenID)
	save()
}

// ClearAll is called on engine startup to wipe stale state
func ClearAll() {
	mu.Lock()
	defer mu.Unlock()
	
	stateMap = make(map[string]StreamerState)
	save()
}

// Intentionally internal function, expects caller to hold mutex
func save() {
	data := map[string]interface{}{
		"active_streamers": stateMap,
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err == nil {
		os.WriteFile(filename, b, 0644)
	}
}
