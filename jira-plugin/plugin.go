package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	sdkv2 "github.com/sorenhq/go-plugin-sdk/gosdk"
	models "github.com/sorenhq/go-plugin-sdk/gosdk/models"

	"github.com/sorenhq/jira-plugin/actions/issues"
	"github.com/sorenhq/jira-plugin/actions/projects"
)

var PluginInstance *sdkv2.Plugin

func main() {
	err := godotenv.Overload("./env.plugin")
	if err != nil {
		fmt.Println(err)
	}
	sdkInstance, err := sdkv2.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to create SDK: %v", err)
	}

	// Debug: Check if auth key and event channel are loaded
	authKey := os.Getenv("SOREN_AUTH_KEY")
	eventChannel := os.Getenv("SOREN_EVENT_CHANNEL")
	if authKey == "" {
		log.Printf("Warning: SOREN_AUTH_KEY is not set or empty")
	} else {
		log.Printf("SOREN_AUTH_KEY is set (length: %d)", len(authKey))
	}
	if eventChannel == "" {
		log.Printf("Warning: SOREN_EVENT_CHANNEL is not set or empty")
	} else {
		log.Printf("SOREN_EVENT_CHANNEL is set: %s", eventChannel)
	}
	defer sdkInstance.Close()

	plugin := sdkv2.NewPlugin(sdkInstance)
	PluginInstance = plugin

	// Set up plugin intro with onboarding requirements
	plugin.SetIntro(models.PluginIntro{
		Name:    "Jira Plugin",
		Version: "1.0.0",
		Author:  "Soren Team",
		Requirements: &models.Requirements{
			ReplyTo: "onboarding",
			Jsonui: map[string]any{
				"type": "VerticalLayout",
				"elements": []map[string]any{
					{
						"type":  "Control",
						"scope": "#/properties/instanceUrl",
					},
					{
						"type":  "Control",
						"scope": "#/properties/email",
					},
					{
						"type":  "Control",
						"scope": "#/properties/apiToken",
					},
				},
			},
			Jsonschema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"instanceUrl": map[string]any{
						"type":        "string",
						"title":       "Jira Instance URL",
						"description": "Your Jira instance URL (e.g., https://yourcompany.atlassian.net)",
					},
					"email": map[string]any{
						"type":        "string",
						"title":       "Email Address",
						"description": "Your Jira account email address",
					},
					"apiToken": map[string]any{
						"type":        "string",
						"title":       "API Token",
						"description": "Your Jira API token (create one at https://id.atlassian.com/manage-profile/security/api-tokens)",
						"format":      "password",
					},
				},
				"required": []string{"instanceUrl", "email", "apiToken"},
			},
		},
	}, onboardingHandler)

	// Collect all actions from different modules
	var allActions []models.Action
	allActions = append(allActions, projects.GetActions()...)
	allActions = append(allActions, issues.GetActions()...)

	// Add all actions to the plugin
	plugin.AddActions(allActions)

	plugin.Start()
}
