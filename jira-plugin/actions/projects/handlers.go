package projects

import (
	"fmt"
	"log"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
	sdkv2 "github.com/sorenhq/go-plugin-sdk/gosdk"
	sdkv2Models "github.com/sorenhq/go-plugin-sdk/gosdk/models"

	"github.com/sorenhq/jira-plugin/client"
	"github.com/sorenhq/jira-plugin/credentials"
)

type metaRequest struct {
	FormData map[string]any `json:"formData"`
	Form     *struct {
		Jsonui     map[string]any `json:"jsonui"`
		Jsonschema map[string]any `json:"jsonschema"`
	} `json:"form"`
}

// metaResponseSuccess returns formData and form encapsulated under top-level "data". Shape: { "data": { "formData": {...}, "form": {...} }, "error": null }.
func metaResponseSuccess(formData map[string]any, form map[string]any) map[string]any {
	if formData == nil {
		formData = map[string]any{}
	}
	return map[string]any{
		"data": map[string]any{
			"formData": formData,
			"form":     form,
		},
		"error": nil,
	}
}

// metaResponseError returns the standard error shape for meta handlers (for frontend handling). error is always a string.
func metaResponseError(msg string) map[string]any {
	return map[string]any{"data": nil, "formData": nil, "form": nil, "error": msg}
}

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
			sdkv2.RejectReq(msg, map[string]any{
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
		sdkv2.RejectReq(msg, map[string]any{
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
		sdkv2.RejectReq(msg, map[string]any{
			"error":   "credentials_error",
			"message": fmt.Sprintf("Failed to retrieve credentials: %v", err),
		})
		return
	}

	// Handshake via SDK (stores entityId and responds)
	jobID := sdkv2.AcceptReq(msg)
	if jobID == "" {
		sdkv2.RejectReq(msg, map[string]any{
			"error":   "job_creation_failed",
			"message": "Failed to create job",
		})
		return
	}

	// Execute and complete
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

func ListProjectsMetaHandler(msg *nats.Msg) {
	var req metaRequest
	if len(msg.Data) > 0 {
		if err := sonic.Unmarshal(msg.Data, &req); err != nil {
			log.Printf("_projects.list: failed to unmarshal request: %v", err)
			resp, _ := sonic.Marshal(metaResponseError("invalid_request"))
			msg.Respond(resp)
			return
		}
	}

	spaceID := extractSpaceIdFromSubject(msg.Subject)
	credsStorage := credentials.GetCredentialsStorage()
	if !credsStorage.HasCredentials(spaceID) {
		resp, _ := sonic.Marshal(metaResponseError("Jira credentials not configured. Please complete onboarding."))
		msg.Respond(resp)
		return
	}
	creds, err := credsStorage.GetCredentials(spaceID)
	if err != nil {
		resp, _ := sonic.Marshal(metaResponseError(fmt.Sprintf("Failed to retrieve credentials: %v", err)))
		msg.Respond(resp)
		return
	}

	jiraClient := client.NewJiraClient(creds)
	projects, err := jiraClient.ListProjects()
	if err != nil {
		resp, _ := sonic.Marshal(metaResponseError(fmt.Sprintf("Failed to fetch projects: %v", err)))
		msg.Respond(resp)
		return
	}

	keys := make([]string, 0, len(projects))
	for _, p := range projects {
		if k, _ := p["key"].(string); k != "" {
			keys = append(keys, k)
		}
	}

	form := req.Form
	if form == nil || form.Jsonschema == nil {
		resp, _ := sonic.Marshal(metaResponseError("missing form in request"))
		msg.Respond(resp)
		return
	}

	jsonschema := make(map[string]any)
	for k, v := range form.Jsonschema {
		jsonschema[k] = v
	}
	props, _ := jsonschema["properties"].(map[string]any)
	if props == nil {
		props = make(map[string]any)
		jsonschema["properties"] = props
	} else {
		props = copyMap(props)
		jsonschema["properties"] = props
	}
	projectKeyProp, _ := props["projectKey"].(map[string]any)
	if projectKeyProp == nil {
		projectKeyProp = make(map[string]any)
	} else {
		projectKeyProp = copyMap(projectKeyProp)
	}
	projectKeyProp["enum"] = keys
	props["projectKey"] = projectKeyProp

	updatedForm := map[string]any{
		"jsonui":     form.Jsonui,
		"jsonschema": jsonschema,
	}
	out := metaResponseSuccess(req.FormData, updatedForm)
	resp, _ := sonic.Marshal(out)
	msg.Respond(resp)
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ListIssueTypesMetaHandler handles POST with { data, Form }. issueType has dependsOn: ["projectKey"]; we read data.projectKey (set by frontend). Returns same format as request with only Form updated.
func ListIssueTypesMetaHandler(msg *nats.Msg) {
	var req metaRequest
	if len(msg.Data) > 0 {
		if err := sonic.Unmarshal(msg.Data, &req); err != nil {
			log.Printf("_issueTypes.list: failed to unmarshal request: %v", err)
			resp, _ := sonic.Marshal(metaResponseError("invalid_request"))
			msg.Respond(resp)
			return
		}
	}

	// dependsOn: frontend sends current form values in formData; projectKey should already be set
	projectKey, _ := req.FormData["projectKey"].(string)
	if projectKey == "" {
		resp, _ := sonic.Marshal(metaResponseError("projectKey is required (dependsOn); ensure it is set in formData before calling _issueTypes.list"))
		msg.Respond(resp)
		return
	}

	spaceID := extractSpaceIdFromSubject(msg.Subject)
	credsStorage := credentials.GetCredentialsStorage()
	if !credsStorage.HasCredentials(spaceID) {
		resp, _ := sonic.Marshal(metaResponseError("Jira credentials not configured."))
		msg.Respond(resp)
		return
	}
	creds, err := credsStorage.GetCredentials(spaceID)
	if err != nil {
		resp, _ := sonic.Marshal(metaResponseError(fmt.Sprintf("Failed to retrieve credentials: %v", err)))
		msg.Respond(resp)
		return
	}

	jiraClient := client.NewJiraClient(creds)
	names, err := jiraClient.GetIssueTypes(projectKey)
	if err != nil {
		log.Printf("GetIssueTypes failed: %v", err)
		resp, _ := sonic.Marshal(metaResponseError(fmt.Sprintf("Failed to fetch issue types: %v", err)))
		msg.Respond(resp)
		return
	}

	form := req.Form
	if form == nil || form.Jsonschema == nil {
		resp, _ := sonic.Marshal(metaResponseError("missing form in request"))
		msg.Respond(resp)
		return
	}

	jsonschema := make(map[string]any)
	for k, v := range form.Jsonschema {
		jsonschema[k] = v
	}
	props, _ := jsonschema["properties"].(map[string]any)
	if props == nil {
		props = make(map[string]any)
		jsonschema["properties"] = props
	} else {
		props = copyMap(props)
		jsonschema["properties"] = props
	}
	issueTypeProp, _ := props["issueType"].(map[string]any)
	if issueTypeProp == nil {
		issueTypeProp = make(map[string]any)
	} else {
		issueTypeProp = copyMap(issueTypeProp)
	}
	issueTypeProp["enum"] = names
	props["issueType"] = issueTypeProp

	updatedForm := map[string]any{
		"jsonui":     form.Jsonui,
		"jsonschema": jsonschema,
	}
	out := metaResponseSuccess(req.FormData, updatedForm)
	resp, _ := sonic.Marshal(out)
	msg.Respond(resp)
}
