package issues

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	sdkv2 "github.com/sorenhq/go-plugin-sdk/gosdk"
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
			sdkv2.RejectWithBody(msg, map[string]any{
				"error":   "invalid_request",
				"message": "Failed to parse request",
			})
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
		sdkv2.RejectWithBody(msg, map[string]any{
			"error":   "credentials_not_configured",
			"message": errorMsg,
			"action":  actionName,
			"spaceId": spaceID,
		})
		return
	}

	// Get credentials
	creds, err := credsStorage.GetCredentials(spaceID)
	if err != nil {
		log.Printf("Failed to get credentials: %v", err)
		sdkv2.RejectWithBody(msg, map[string]any{
			"error":   "credentials_error",
			"message": fmt.Sprintf("Failed to retrieve credentials: %v", err),
		})
		return
	}

	// 1. Handshake
	jobID := uuid.New().String()
	initResponse := map[string]any{
		"jobId":    jobID,
		"progress": 0,
	}
	if plugin := sdkv2.GetPlugin(); plugin != nil {
		plugin.StoreEntityIdForJob(jobID, spaceID)
	}
	response, err := sonic.Marshal(initResponse)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		sdkv2.RejectWithBody(msg, map[string]any{
			"error":   "internal_error",
			"message": "Failed to serialize response",
		})
		return
	}
	msg.Respond(response)

	// 2. WAIT (Crucial for NATS state synchronization)
	time.Sleep(1 * time.Second)

	// 3. COMPLETE
	result := actionFunc(creds, body)
	if plugin := sdkv2.GetPlugin(); plugin != nil {
		plugin.Done(jobID, result)
	} else {
		log.Printf("Failed to publish result: plugin instance not found")
	}
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
