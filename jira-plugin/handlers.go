package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"

	"github.com/sorenhq/jira-plugin/credentials"
)

// onboardingHandler handles the onboarding/requirements submission
func onboardingHandler(msg *nats.Msg) any {
	// Extract spaceId from the NATS message subject
	spaceID := extractSpaceIdFromSubject(msg.Subject)
	log.Printf("Onboarding request received for space '%s' (extracted from subject: %s)", spaceID, msg.Subject)

	var onboardingData map[string]any
	err := sonic.Unmarshal(msg.Data, &onboardingData)
	if err != nil {
		log.Printf("Failed to unmarshal onboarding data: %v", err)
		response, _ := json.Marshal(map[string]any{
			"status": "error",
			"error":  "Invalid request data",
		})
		msg.Respond(response)
		return nil
	}

	// Extract credentials from onboarding data
	creds := credentials.JiraCredentials{
		InstanceURL: getStringValue(onboardingData, "instanceUrl"),
		Email:       getStringValue(onboardingData, "email"),
		APIToken:    getStringValue(onboardingData, "apiToken"),
	}

	// Validate required fields
	if creds.InstanceURL == "" || creds.Email == "" || creds.APIToken == "" {
		response, _ := json.Marshal(map[string]any{
			"status": "error",
			"error":  "Missing required fields: instanceUrl, email, and apiToken are required",
		})
		msg.Respond(response)
		return nil
	}

	// Save credentials using spaceID as the key
	credsStorage := credentials.GetCredentialsStorage()
	err = credsStorage.SaveCredentials(spaceID, creds)
	if err != nil {
		log.Printf("Failed to save credentials: %v", err)
		response, _ := json.Marshal(map[string]any{
			"status": "error",
			"error":  fmt.Sprintf("Failed to save credentials: %v", err),
		})
		msg.Respond(response)
		return nil
	}

	log.Printf("Credentials saved successfully for space: %s", spaceID)
	response, _ := json.Marshal(map[string]any{
		"status":  "accepted",
		"message": "Credentials saved successfully",
	})
	msg.Respond(response)
	return nil
}

// extractSpaceIdFromSubject extracts the entityId (spaceId) from NATS message subject
// Subject pattern: soren.v2.bin.{entityId}.{pluginId}.{path} or soren.cpu.bin.{entityId}.{pluginId}.{path}
func extractSpaceIdFromSubject(subject string) string {
	parts := strings.Split(subject, ".")
	// Look for "bin" in the subject, entityId should be right after it
	for i, part := range parts {
		if part == "bin" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	// If pattern doesn't match, return empty string (will use default)
	return ""
}

// getStringValue safely extracts a string value from a map
func getStringValue(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
