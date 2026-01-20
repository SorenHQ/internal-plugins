package projects

import (
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
	sdkv2Models "github.com/sorenhq/go-plugin-sdk/gosdk/models"

	"github.com/sorenhq/jira-plugin/client"
	"github.com/sorenhq/jira-plugin/credentials"
)

// GetActions returns all project-related actions
func GetActions() []sdkv2Models.Action {
	return []sdkv2Models.Action{
		{
			Method:      "projects.list",
			Title:       "List Projects",
			Description: "Get a list of all projects in your Jira instance",
			Form: sdkv2Models.ActionFormBuilder{
				Jsonui:     map[string]any{},
				Jsonschema: map[string]any{"type": "object", "properties": map[string]any{}},
			},
			RequestHandler: ListProjectsHandler,
		},
	}
}

// ListProjectsHandler handles the projects.list action
func ListProjectsHandler(msg *nats.Msg) {
	handleActionWithCredentialsCheckSync(msg, "projects.list", func(creds *credentials.JiraCredentials, body map[string]any) map[string]any {
		// Create Jira client and fetch projects
		jiraClient := client.NewJiraClient(creds)
		projects, err := jiraClient.ListProjects()
		if err != nil {
			log.Printf("Failed to list projects: %v", err)
			return map[string]any{
				"error":   "jira_api_error",
				"message": fmt.Sprintf("Failed to fetch projects: %v", err),
			}
		}

		log.Printf("Successfully retrieved %d projects from Jira", len(projects))
		if len(projects) > 0 {
			log.Printf("First project: %+v", projects[0])
		}

		result := map[string]any{
			"result":   "success",
			"message":  fmt.Sprintf("Successfully retrieved %d projects", len(projects)),
			"projects": projects,
			"count":    len(projects),
		}
		return result
	})
}
