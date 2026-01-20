package issues

import (
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
	sdkv2Models "github.com/sorenhq/go-plugin-sdk/gosdk/models"

	"github.com/sorenhq/jira-plugin/client"
	"github.com/sorenhq/jira-plugin/credentials"
)

// GetActions returns all issue-related actions
func GetActions() []sdkv2Models.Action {
	return []sdkv2Models.Action{
		{
			Method:      "issues.create",
			Title:       "Create Issue",
			Description: "Create a new issue in Jira",
			Form: sdkv2Models.ActionFormBuilder{
				Jsonui: map[string]any{
					"type": "VerticalLayout",
					"elements": []map[string]any{
						{
							"type":  "Control",
							"scope": "#/properties/projectKey",
						},
						{
							"type":  "Control",
							"scope": "#/properties/issueType",
						},
						{
							"type":  "Control",
							"scope": "#/properties/summary",
						},
						{
							"type":  "Control",
							"scope": "#/properties/description",
						},
						{
							"type":  "Control",
							"scope": "#/properties/additionalFields",
							"options": map[string]any{
								"format": "json",
							},
						},
					},
				},
				Jsonschema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"projectKey": map[string]any{
							"type":        "string",
							"title":       "Project Key",
							"description": "The project key (e.g., PROJ)",
						},
						"issueType": map[string]any{
							"type":        "string",
							"title":       "Issue Type",
							"description": "Type of issue (e.g., Task, Bug, Story)",
							"enum":        []string{"Task", "Bug", "Story", "Epic"},
						},
						"summary": map[string]any{
							"type":        "string",
							"title":       "Summary",
							"description": "Issue summary/title",
						},
						"description": map[string]any{
							"type":        "string",
							"title":       "Description",
							"description": "Issue description",
						},
						"additionalFields": map[string]any{
							"type":                 "object",
							"title":                "Additional Fields",
							"description":          "Additional Jira fields as key-value pairs (JSON object). Examples: {\"duedate\": \"2024-12-31\"}, {\"priority\": {\"name\": \"High\"}}, {\"assignee\": {\"accountId\": \"user-id\"}}. Field names should match Jira field IDs or names.",
							"additionalProperties": true,
						},
					},
					"required":             []string{"projectKey", "issueType", "summary"},
					"additionalProperties": true, // Allow any additional properties for flexibility
				},
			},
			RequestHandler: CreateIssueHandler,
		},
		{
			Method:      "issues.delete",
			Title:       "Delete Issue",
			Description: "Delete an issue from Jira by issue key or ID",
			Form: sdkv2Models.ActionFormBuilder{
				Jsonui: map[string]any{
					"type": "VerticalLayout",
					"elements": []map[string]any{
						{
							"type":  "Control",
							"scope": "#/properties/issueKey",
						},
						{
							"type":  "Control",
							"scope": "#/properties/deleteSubtasks",
						},
					},
				},
				Jsonschema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"issueKey": map[string]any{
							"type":        "string",
							"title":       "Issue Key or ID",
							"description": "The issue key (e.g., COM-123) or issue ID",
						},
						"deleteSubtasks": map[string]any{
							"type":        "boolean",
							"title":       "Delete Subtasks",
							"description": "If true, delete subtasks when deleting the issue",
							"default":     false,
						},
					},
					"required": []string{"issueKey"},
				},
			},
			RequestHandler: DeleteIssueHandler,
		},
		{
			Method:      "issues.comment",
			Title:       "Add Comment",
			Description: "Add a comment to a Jira issue",
			Form: sdkv2Models.ActionFormBuilder{
				Jsonui: map[string]any{
					"type": "VerticalLayout",
					"elements": []map[string]any{
						{
							"type":  "Control",
							"scope": "#/properties/issueKey",
						},
						{
							"type":  "Control",
							"scope": "#/properties/commentBody",
						},
						{
							"type":  "Control",
							"scope": "#/properties/visibility",
						},
						{
							"type":  "Control",
							"scope": "#/properties/additionalFields",
							"options": map[string]any{
								"format": "json",
							},
						},
					},
				},
				Jsonschema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"issueKey": map[string]any{
							"type":        "string",
							"title":       "Issue Key or ID",
							"description": "The issue key (e.g., COM-123) or issue ID",
						},
						"commentBody": map[string]any{
							"type":        "string",
							"title":       "Comment",
							"description": "The comment text to add",
							"format":      "textarea",
						},
						"visibility": map[string]any{
							"type":        "object",
							"title":       "Visibility (Optional)",
							"description": "Comment visibility settings. Example: {\"type\": \"role\", \"value\": \"Administrators\"} or {\"type\": \"group\", \"value\": \"jira-developers\"}",
							"properties": map[string]any{
								"type": map[string]any{
									"type":        "string",
									"title":       "Visibility Type",
									"description": "Type of visibility: 'role' or 'group'",
									"enum":        []string{"role", "group"},
								},
								"value": map[string]any{
									"type":        "string",
									"title":       "Visibility Value",
									"description": "Role or group name",
								},
							},
						},
						"additionalFields": map[string]any{
							"type":                 "object",
							"title":                "Additional Fields",
							"description":          "Additional Jira comment fields as key-value pairs (JSON object). Can be used for custom fields or future Jira API extensions.",
							"additionalProperties": true,
						},
					},
					"required":             []string{"issueKey", "commentBody"},
					"additionalProperties": true, // Allow any additional properties for flexibility
				},
			},
			RequestHandler: AddCommentHandler,
		},
	}
}

// CreateIssueHandler handles the issues.create action
func CreateIssueHandler(msg *nats.Msg) {
	handleActionWithCredentialsCheckSync(msg, "issues.create", func(creds *credentials.JiraCredentials, body map[string]any) map[string]any {
		// Extract core form fields
		projectKey, _ := body["projectKey"].(string)
		issueType, _ := body["issueType"].(string)
		summary, _ := body["summary"].(string)
		description, _ := body["description"].(string)

		// Extract additionalFields if provided (as object)
		var additionalFields map[string]interface{}
		if additionalFieldsRaw, ok := body["additionalFields"]; ok {
			if afMap, ok := additionalFieldsRaw.(map[string]interface{}); ok {
				additionalFields = afMap
			} else if afMap, ok := additionalFieldsRaw.(map[string]any); ok {
				// Convert map[string]any to map[string]interface{}
				additionalFields = make(map[string]interface{})
				for k, v := range afMap {
					additionalFields[k] = v
				}
			}
		}

		// Also check for any other fields that might have been passed directly
		// (for backward compatibility and flexibility)
		knownFields := map[string]bool{
			"projectKey":       true,
			"issueType":        true,
			"summary":          true,
			"description":      true,
			"additionalFields": true,
		}

		// Merge any other fields that aren't in the known list into additionalFields
		if additionalFields == nil {
			additionalFields = make(map[string]interface{})
		}
		for key, value := range body {
			if !knownFields[key] && value != nil && value != "" {
				additionalFields[key] = value
			}
		}

		// Validate required fields
		if projectKey == "" {
			return map[string]any{
				"error":   "validation_error",
				"message": "Project key is required",
			}
		}
		if issueType == "" {
			return map[string]any{
				"error":   "validation_error",
				"message": "Issue type is required",
			}
		}
		if summary == "" {
			return map[string]any{
				"error":   "validation_error",
				"message": "Summary is required",
			}
		}

		// Create Jira client and create issue
		jiraClient := client.NewJiraClient(creds)
		issue, err := jiraClient.CreateIssue(projectKey, issueType, summary, description, additionalFields)
		if err != nil {
			log.Printf("Failed to create issue: %v", err)
			return map[string]any{
				"error":   "jira_api_error",
				"message": fmt.Sprintf("Failed to create issue: %v", err),
			}
		}

		// Extract issue key from response
		issueKey, _ := issue["key"].(string)
		issueId, _ := issue["id"].(string)

		log.Printf("Successfully created Jira issue: %s (ID: %s)", issueKey, issueId)

		result := map[string]any{
			"result":   "success",
			"message":  "Issue created successfully",
			"issueKey": issueKey,
			"issueId":  issueId,
			"issue":    issue,
		}
		return result
	})
}

// DeleteIssueHandler handles the issues.delete action
func DeleteIssueHandler(msg *nats.Msg) {
	handleActionWithCredentialsCheckSync(msg, "issues.delete", func(creds *credentials.JiraCredentials, body map[string]any) map[string]any {
		// Extract form fields
		issueKey, _ := body["issueKey"].(string)
		deleteSubtasks := false
		if ds, ok := body["deleteSubtasks"].(bool); ok {
			deleteSubtasks = ds
		}

		// Validate required fields
		if issueKey == "" {
			return map[string]any{
				"error":   "validation_error",
				"message": "Issue key or ID is required",
			}
		}

		// Create Jira client and delete issue
		jiraClient := client.NewJiraClient(creds)
		err := jiraClient.DeleteIssue(issueKey, deleteSubtasks)
		if err != nil {
			log.Printf("Failed to delete issue: %v", err)
			return map[string]any{
				"error":   "jira_api_error",
				"message": fmt.Sprintf("Failed to delete issue: %v", err),
			}
		}

		log.Printf("Successfully deleted Jira issue: %s", issueKey)

		result := map[string]any{
			"result":   "success",
			"message":  fmt.Sprintf("Issue %s deleted successfully", issueKey),
			"issueKey": issueKey,
		}
		return result
	})
}

// AddCommentHandler handles the issues.comment action
func AddCommentHandler(msg *nats.Msg) {
	handleActionWithCredentialsCheckSync(msg, "issues.comment", func(creds *credentials.JiraCredentials, body map[string]any) map[string]any {
		// Extract form fields
		issueKey, _ := body["issueKey"].(string)
		commentBody, _ := body["commentBody"].(string)
		var visibility map[string]interface{}

		// Extract visibility if provided
		if visibilityRaw, ok := body["visibility"]; ok {
			if visMap, ok := visibilityRaw.(map[string]interface{}); ok {
				visibility = visMap
			} else if visMap, ok := visibilityRaw.(map[string]any); ok {
				// Convert map[string]any to map[string]interface{}
				visibility = make(map[string]interface{})
				for k, v := range visMap {
					visibility[k] = v
				}
			}
		}

		// Extract additionalFields if provided (as object)
		var additionalFields map[string]interface{}
		if additionalFieldsRaw, ok := body["additionalFields"]; ok {
			if afMap, ok := additionalFieldsRaw.(map[string]interface{}); ok {
				additionalFields = afMap
			} else if afMap, ok := additionalFieldsRaw.(map[string]any); ok {
				// Convert map[string]any to map[string]interface{}
				additionalFields = make(map[string]interface{})
				for k, v := range afMap {
					additionalFields[k] = v
				}
			}
		}

		// Also check for any other fields that might have been passed directly
		// (for backward compatibility and flexibility)
		knownFields := map[string]bool{
			"issueKey":         true,
			"commentBody":      true,
			"visibility":       true,
			"additionalFields": true,
		}

		// Merge any other fields that aren't in the known list into additionalFields
		if additionalFields == nil {
			additionalFields = make(map[string]interface{})
		}
		for key, value := range body {
			if !knownFields[key] && value != nil && value != "" {
				additionalFields[key] = value
			}
		}

		// Validate required fields
		if issueKey == "" {
			return map[string]any{
				"error":   "validation_error",
				"message": "Issue key or ID is required",
			}
		}
		if commentBody == "" {
			return map[string]any{
				"error":   "validation_error",
				"message": "Comment body is required",
			}
		}

		// Create Jira client and add comment
		jiraClient := client.NewJiraClient(creds)
		comment, err := jiraClient.AddComment(issueKey, commentBody, visibility, additionalFields)
		if err != nil {
			log.Printf("Failed to add comment: %v", err)
			return map[string]any{
				"error":   "jira_api_error",
				"message": fmt.Sprintf("Failed to add comment: %v", err),
			}
		}

		// Extract comment ID from response
		commentId, _ := comment["id"].(string)
		commentAuthor, _ := comment["author"].(map[string]interface{})

		log.Printf("Successfully added comment to Jira issue %s (comment ID: %s)", issueKey, commentId)

		result := map[string]any{
			"result":        "success",
			"message":       fmt.Sprintf("Comment added successfully to issue %s", issueKey),
			"issueKey":      issueKey,
			"commentId":     commentId,
			"comment":       comment,
			"commentAuthor": commentAuthor,
		}
		return result
	})
}
