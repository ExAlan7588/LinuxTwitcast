package discord

import "testing"

func TestResolveScreenIDFromTargetMessageUsesEmbedURL(t *testing.T) {
	data := interactionCommandData{
		TargetID: "message-1",
		Resolved: interactionResolved{
			Messages: map[string]resolvedMessage{
				"message-1": {
					Embeds: []embed{
						{Url: "https://twitcasting.tv/streamer/show/12345"},
					},
				},
			},
		},
	}

	screenID, err := resolveScreenIDFromTargetMessage(data)
	if err != nil {
		t.Fatalf("resolveScreenIDFromTargetMessage returned error: %v", err)
	}
	if screenID != "streamer" {
		t.Fatalf("screenID = %q, want %q", screenID, "streamer")
	}
}

func TestResolveScreenIDFromTargetMessageRejectsNonTwitCastingURL(t *testing.T) {
	data := interactionCommandData{
		TargetID: "message-1",
		Resolved: interactionResolved{
			Messages: map[string]resolvedMessage{
				"message-1": {
					Embeds: []embed{
						{Url: "https://example.com/streamer"},
					},
				},
			},
		},
	}

	if _, err := resolveScreenIDFromTargetMessage(data); err == nil {
		t.Fatal("expected error for non-TwitCasting URL")
	}
}

func TestResolveScreenIDFromTargetMessageFallsBackToAuthorURL(t *testing.T) {
	data := interactionCommandData{
		TargetID: "message-1",
		Resolved: interactionResolved{
			Messages: map[string]resolvedMessage{
				"message-1": {
					Embeds: []embed{
						{
							Author: &author{Url: "https://twitcasting.tv/streamer"},
						},
					},
				},
			},
		},
	}

	screenID, err := resolveScreenIDFromTargetMessage(data)
	if err != nil {
		t.Fatalf("resolveScreenIDFromTargetMessage returned error: %v", err)
	}
	if screenID != "streamer" {
		t.Fatalf("screenID = %q, want %q", screenID, "streamer")
	}
}
