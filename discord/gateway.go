package discord

import (
	"context"
	"encoding/json"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const gatewayURL = "wss://gateway.discord.gg/?v=10&encoding=json"

// gatewayPayload is a generic Discord Gateway message envelope.
type gatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  *int            `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

// Gateway maintains a persistent Discord Gateway WebSocket connection.
// It handles heartbeating, IDENTIFY, and dispatches INTERACTION_CREATE events
// to the interaction handler.
type Gateway struct {
	cfg   Config
	appID string

	mu   sync.Mutex
	conn *websocket.Conn
	seq  *int
}

// NewGateway creates a Gateway for the given configuration.
// appID is the bot's Discord application ID (obtained via FetchAppID).
func NewGateway(cfg Config, appID string) *Gateway {
	return &Gateway{cfg: cfg, appID: appID}
}

// Run connects to the Gateway and reconnects automatically until ctx is cancelled.
// Intended to be called as a goroutine.
func (g *Gateway) Run(ctx context.Context) {
	for {
		if err := g.runOnce(ctx); err != nil {
			if ctx.Err() != nil {
				log.Println("[Gateway] Shutting down")
				return
			}
			log.Printf("[Gateway] Disconnected: %v. Reconnecting in 10s...\n", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

// runOnce opens one WebSocket session and handles it until an error occurs or ctx is cancelled.
func (g *Gateway) runOnce(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, gatewayURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	g.mu.Lock()
	g.conn = conn
	g.mu.Unlock()

	// cancelHb cancels the heartbeat goroutine when this session ends
	hbCtx, cancelHb := context.WithCancel(ctx)
	defer cancelHb()

	for {
		// Check if context was cancelled (program terminating)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var p gatewayPayload
		if err := json.Unmarshal(msg, &p); err != nil {
			continue
		}

		// Track sequence number for heartbeat
		if p.S != nil {
			g.mu.Lock()
			g.seq = p.S
			g.mu.Unlock()
		}

		switch p.Op {
		case 10: // HELLO — start heartbeating then identify
			var hello struct {
				HeartbeatInterval int `json:"heartbeat_interval"`
			}
			json.Unmarshal(p.D, &hello) //nolint:errcheck
			go g.heartbeat(hbCtx, hello.HeartbeatInterval)
			g.identify()

		case 11: // HEARTBEAT_ACK
			// All good, nothing to do

		case 9: // INVALID SESSION — re-identify after a short delay
			log.Println("[Gateway] Invalid session received, re-identifying...")
			time.Sleep(2 * time.Second)
			g.identify()

		case 1: // HEARTBEAT request from server
			g.sendHeartbeat()

		case 0: // DISPATCH — dispatched event
			go g.handleDispatch(p.T, p.D)
		}
	}
}

// writeJSON serialises v and sends it over the WebSocket (thread-safe).
func (g *Gateway) writeJSON(v interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.conn != nil {
		if err := g.conn.WriteJSON(v); err != nil {
			log.Printf("[Gateway] Write error: %v\n", err)
		}
	}
}

func (g *Gateway) sendHeartbeat() {
	g.mu.Lock()
	seq := g.seq
	g.mu.Unlock()
	g.writeJSON(map[string]interface{}{"op": 1, "d": seq})
}

func (g *Gateway) heartbeat(ctx context.Context, intervalMs int) {
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			g.sendHeartbeat()
		case <-ctx.Done():
			return
		}
	}
}

func (g *Gateway) identify() {
	g.writeJSON(map[string]interface{}{
		"op": 2,
		"d": map[string]interface{}{
			"token":   "Bot " + g.cfg.BotToken,
			"intents": 0, // No privileged intents needed for interactions
			"properties": map[string]interface{}{
				"os":      runtime.GOOS,
				"browser": "croned-twitcasting-recorder",
				"device":  "croned-twitcasting-recorder",
			},
		},
	})
}

func (g *Gateway) handleDispatch(event string, data json.RawMessage) {
	switch event {
	case "READY":
		var ready struct {
			SessionID string `json:"session_id"`
			User      struct {
				Username string `json:"username"`
			} `json:"user"`
		}
		json.Unmarshal(data, &ready) //nolint:errcheck
		log.Printf("[Gateway] Connected as %s (session %s)\n", ready.User.Username, ready.SessionID)

		// Register the context menu command now that we have a valid session
		RegisterContextMenuCommand(g.cfg, g.appID)

	case "INTERACTION_CREATE":
		HandleInteraction(g.cfg, g.appID, data)
	}
}
