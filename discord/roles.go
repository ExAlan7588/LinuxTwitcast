package discord

import (
	"encoding/json"
	"fmt"
	"log"
)

type discordGuildRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetOrCreateRoleByScreenID looks up a guild role named screenID and creates it
// (mentionable) if it does not exist. Returns "" on failure.
func GetOrCreateRoleByScreenID(botToken, guildID, screenID string) string {
	if botToken == "" || guildID == "" || screenID == "" {
		return ""
	}

	roleID, err := findGuildRoleIDByName(botToken, guildID, screenID)
	if err == nil {
		return roleID
	}

	log.Printf("[Discord] Failed to fetch guild roles for '%s': %v\n", screenID, err)
	return ""
}

func findGuildRoleIDByName(botToken, guildID, roleName string) (string, error) {
	roles, err := fetchGuildRoles(botToken, guildID)
	if err != nil {
		return "", err
	}

	for _, role := range roles {
		if role.Name == roleName {
			return role.ID, nil
		}
	}

	return createGuildRole(botToken, guildID, roleName)
}

func fetchGuildRoles(botToken, guildID string) ([]discordGuildRole, error) {
	body, status, err := apiCall(botToken, "GET", "/guilds/"+guildID+"/roles", nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("discord API error %d: %s", status, string(body))
	}

	var roles []discordGuildRole
	if err := json.Unmarshal(body, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

func createGuildRole(botToken, guildID, roleName string) (string, error) {
	body, status, err := apiCall(botToken, "POST", "/guilds/"+guildID+"/roles",
		rolePayload{Name: roleName, Mentionable: true})
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("discord API error %d: %s", status, string(body))
	}

	var role discordGuildRole
	if err := json.Unmarshal(body, &role); err != nil {
		return "", err
	}
	if role.ID == "" {
		return "", fmt.Errorf("role ID missing in response")
	}

	log.Printf("[Discord] Created role '%s' (id=%s)\n", roleName, role.ID)
	return role.ID, nil
}
