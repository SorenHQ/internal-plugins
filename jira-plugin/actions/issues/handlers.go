package issues

import (
	"fmt"
	"log"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
	sdkv2Models "github.com/sorenhq/go-plugin-sdk/gosdk/models"

	"github.com/sorenhq/jira-plugin/credentials"
)

// handleActionWithCredentialsCheckSync is a helper function for synchronous actions that respond directly
func handleActionWithCredentialsCheckSync(msg *nats.Msg, actionName string, actionFunc func(*credentials.JiraCredentials, map[string]any) map[string]any) {
	// Extract spaceId from the NATS message subject
	spaceID := extractSpaceIdFromSubject(msg.Subject)
	log.Printf("Action %s called for space '%s' (extracted from subject: %s)", actionName, spaceID, msg.Subject)
	log.Printf("Message data length: %d bytes, content: %s", len(msg.Data), string(msg.Data))

	// Handle empty or missing request body (for actions with no form fields)
	var requestData sdkv2Models.ActionRequestContent
	var body map[string]any = make(map[string]any)

	if len(msg.Data) > 0 {
		err := sonic.Unmarshal(msg.Data, &requestData)
		if err != nil {
			log.Printf("Failed to unmarshal action request: %v", err)
			response, _ := sonic.Marshal(map[string]any{
				"error":   "Invalid request data",
				"message": "Failed to parse request",
			})
			msg.Respond(response)
			return
		}
		// Use the body from requestData if available, otherwise use empty map
		if requestData.Body != nil {
			body = requestData.Body
		}
	} else {
		log.Printf("Empty message body for action %s, using empty body map", actionName)
	}

	// Get credentials storage instance
	credsStorage := credentials.GetCredentialsStorage()

	// Check if credentials exist for this space
	if !credsStorage.HasCredentials(spaceID) {
		errorMsg := fmt.Sprintf("Jira credentials not configured for space '%s'. Please complete the onboarding process first.", spaceID)
		if spaceID == "" {
			errorMsg = "Jira credentials not configured. Please complete the onboarding process first."
		}

		log.Printf("Action %s rejected for space '%s': %s", actionName, spaceID, errorMsg)
		response, _ := sonic.Marshal(map[string]any{
			"error":   "credentials_not_configured",
			"message": errorMsg,
			"action":  actionName,
			"spaceId": spaceID,
		})
		msg.Respond(response)
		return
	}

	// Get credentials
	creds, err := credsStorage.GetCredentials(spaceID)
	if err != nil {
		log.Printf("Failed to get credentials: %v", err)
		response, _ := sonic.Marshal(map[string]any{
			"error":   "credentials_error",
			"message": fmt.Sprintf("Failed to retrieve credentials: %v", err),
		})
		msg.Respond(response)
		return
	}

	// Execute the action and get result
	result := actionFunc(creds, body)

	// Respond directly with the result
	response, err := sonic.Marshal(result)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		response, _ = sonic.Marshal(map[string]any{
			"error":   "internal_error",
			"message": "Failed to serialize response",
		})
	}
	msg.Respond(response)
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
