package client

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"

	"github.com/sorenhq/jira-plugin/credentials"
)

// JiraClient handles Jira API calls
type JiraClient struct {
	BaseURL    string
	Email      string
	APIToken   string
	HTTPClient *http.Client
}

// NewJiraClient creates a new Jira API client
func NewJiraClient(creds *credentials.JiraCredentials) *JiraClient {
	return &JiraClient{
		BaseURL:  creds.InstanceURL,
		Email:    creds.Email,
		APIToken: creds.APIToken,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// makeRequest makes an authenticated HTTP request to Jira API
func (jc *JiraClient) makeRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	// Normalize base URL (remove trailing slash) and ensure endpoint starts with /
	baseURL := strings.TrimSuffix(jc.BaseURL, "/")
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	url := fmt.Sprintf("%s%s", baseURL, endpoint)

	log.Printf("Making Jira API request: %s %s", method, url)

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use Bearer token authentication with PAT (Personal Access Token)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jc.APIToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := jc.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}

// ListProjects retrieves all projects from Jira
func (jc *JiraClient) ListProjects() ([]map[string]interface{}, error) {
	resp, err := jc.makeRequest("GET", "/rest/api/2/project", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Jira API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("Jira API response status: %d, body length: %d bytes", resp.StatusCode, len(bodyBytes))
	if len(bodyBytes) > 0 && len(bodyBytes) < 1000 {
		log.Printf("Jira API response body: %s", string(bodyBytes))
	}

	var projects []map[string]interface{}
	err = sonic.Unmarshal(bodyBytes, &projects)
	if err != nil {
		log.Printf("Failed to unmarshal projects response: %v, body: %s", err, string(bodyBytes))
		return nil, fmt.Errorf("failed to unmarshal projects: %w", err)
	}

	log.Printf("Successfully parsed %d projects from Jira API", len(projects))
	return projects, nil
}

// CreateIssue creates a new issue in Jira
func (jc *JiraClient) CreateIssue(projectKey, issueType, summary, description string, additionalFields map[string]interface{}) (map[string]interface{}, error) {
	// Build the request body
	fields := map[string]interface{}{
		"project": map[string]interface{}{
			"key": projectKey,
		},
		"summary": summary,
		"issuetype": map[string]interface{}{
			"name": issueType,
		},
	}

	// Add description if provided
	if description != "" {
		fields["description"] = description
	}

	// Add any additional fields (like duedate, assignee, etc.)
	for key, value := range additionalFields {
		if value != nil && value != "" {
			fields[key] = value
		}
	}

	requestBody := map[string]interface{}{
		"fields": fields,
	}

	// Marshal request body
	bodyBytes, err := sonic.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	log.Printf("Creating Jira issue with body: %s", string(bodyBytes))

	// Create request body reader
	bodyReader := bytes.NewReader(bodyBytes)

	// Make the API call
	resp, err := jc.makeRequest("POST", "/rest/api/2/issue", bodyReader)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("Jira API response status: %d, body length: %d bytes", resp.StatusCode, len(bodyBytes))
	if len(bodyBytes) > 0 && len(bodyBytes) < 1000 {
		log.Printf("Jira API response body: %s", string(bodyBytes))
	}

	// Check for errors
	if resp.StatusCode != http.StatusCreated {
		// Try to parse Jira error response for better error messages
		var jiraError struct {
			ErrorMessages []string          `json:"errorMessages"`
			Errors        map[string]string `json:"errors"`
		}

		if err := sonic.Unmarshal(bodyBytes, &jiraError); err == nil {
			// Build a user-friendly error message
			var errorParts []string

			// Add error messages
			for _, msg := range jiraError.ErrorMessages {
				errorParts = append(errorParts, msg)
			}

			// Add field-specific errors
			if len(jiraError.Errors) > 0 {
				fieldErrors := []string{}
				for field, msg := range jiraError.Errors {
					fieldErrors = append(fieldErrors, fmt.Sprintf("%s: %s", field, msg))
				}
				if len(fieldErrors) > 0 {
					errorParts = append(errorParts, fmt.Sprintf("Missing or invalid fields: %s", strings.Join(fieldErrors, "; ")))
				}
			}

			if len(errorParts) > 0 {
				return nil, fmt.Errorf("Jira API error (status %d): %s", resp.StatusCode, strings.Join(errorParts, ". "))
			}
		}

		// Fallback to raw error message
		return nil, fmt.Errorf("Jira API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var issue map[string]interface{}
	err = sonic.Unmarshal(bodyBytes, &issue)
	if err != nil {
		log.Printf("Failed to unmarshal issue response: %v, body: %s", err, string(bodyBytes))
		return nil, fmt.Errorf("failed to unmarshal issue: %w", err)
	}

	log.Printf("Successfully created Jira issue: %v", issue)
	return issue, nil
}

// DeleteIssue deletes an issue from Jira by issue key or ID
func (jc *JiraClient) DeleteIssue(issueKeyOrId string, deleteSubtasks bool) error {
	// Build the endpoint with optional query parameter
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s", issueKeyOrId)
	if deleteSubtasks {
		endpoint += "?deleteSubtasks=true"
	}

	log.Printf("Deleting Jira issue: %s (deleteSubtasks: %v)", issueKeyOrId, deleteSubtasks)

	// Make the DELETE request
	resp, err := jc.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response body (even for successful deletes, there might be a response)
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("Jira API response status: %d, body length: %d bytes", resp.StatusCode, len(bodyBytes))
	if len(bodyBytes) > 0 && len(bodyBytes) < 1000 {
		log.Printf("Jira API response body: %s", string(bodyBytes))
	}

	// Check for errors (204 No Content is success for DELETE)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		// Try to parse Jira error response for better error messages
		var jiraError struct {
			ErrorMessages []string          `json:"errorMessages"`
			Errors        map[string]string `json:"errors"`
		}

		if err := sonic.Unmarshal(bodyBytes, &jiraError); err == nil {
			// Build a user-friendly error message
			var errorParts []string

			// Add error messages
			for _, msg := range jiraError.ErrorMessages {
				errorParts = append(errorParts, msg)
			}

			// Add field-specific errors
			if len(jiraError.Errors) > 0 {
				fieldErrors := []string{}
				for field, msg := range jiraError.Errors {
					fieldErrors = append(fieldErrors, fmt.Sprintf("%s: %s", field, msg))
				}
				if len(fieldErrors) > 0 {
					errorParts = append(errorParts, fmt.Sprintf("Errors: %s", strings.Join(fieldErrors, "; ")))
				}
			}

			if len(errorParts) > 0 {
				return fmt.Errorf("Jira API error (status %d): %s", resp.StatusCode, strings.Join(errorParts, ". "))
			}
		}

		// Fallback to raw error message
		return fmt.Errorf("Jira API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	log.Printf("Successfully deleted Jira issue: %s", issueKeyOrId)
	return nil
}

// AddComment adds a comment to a Jira issue
func (jc *JiraClient) AddComment(issueKeyOrId, commentBody string, visibility map[string]interface{}, additionalFields map[string]interface{}) (map[string]interface{}, error) {
	// Build the request body
	requestBody := map[string]interface{}{
		"body": commentBody,
	}

	// Add visibility if provided
	if len(visibility) > 0 {
		requestBody["visibility"] = visibility
	}

	// Add any additional fields (for future Jira API extensions or custom fields)
	for key, value := range additionalFields {
		if value != nil && value != "" {
			requestBody[key] = value
		}
	}

	// Marshal request body
	bodyBytes, err := sonic.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	log.Printf("Adding comment to Jira issue %s with body: %s", issueKeyOrId, string(bodyBytes))

	// Create request body reader
	bodyReader := bytes.NewReader(bodyBytes)

	// Build the endpoint
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s/comment", issueKeyOrId)

	// Make the API call
	resp, err := jc.makeRequest("POST", endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("Jira API response status: %d, body length: %d bytes", resp.StatusCode, len(bodyBytes))
	if len(bodyBytes) > 0 && len(bodyBytes) < 1000 {
		log.Printf("Jira API response body: %s", string(bodyBytes))
	}

	// Check for errors (201 Created is success for POST comment)
	if resp.StatusCode != http.StatusCreated {
		// Try to parse Jira error response for better error messages
		var jiraError struct {
			ErrorMessages []string          `json:"errorMessages"`
			Errors        map[string]string `json:"errors"`
		}

		if err := sonic.Unmarshal(bodyBytes, &jiraError); err == nil {
			// Build a user-friendly error message
			var errorParts []string

			// Add error messages
			for _, msg := range jiraError.ErrorMessages {
				errorParts = append(errorParts, msg)
			}

			// Add field-specific errors
			if len(jiraError.Errors) > 0 {
				fieldErrors := []string{}
				for field, msg := range jiraError.Errors {
					fieldErrors = append(fieldErrors, fmt.Sprintf("%s: %s", field, msg))
				}
				if len(fieldErrors) > 0 {
					errorParts = append(errorParts, fmt.Sprintf("Errors: %s", strings.Join(fieldErrors, "; ")))
				}
			}

			if len(errorParts) > 0 {
				return nil, fmt.Errorf("Jira API error (status %d): %s", resp.StatusCode, strings.Join(errorParts, ". "))
			}
		}

		// Fallback to raw error message
		return nil, fmt.Errorf("Jira API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var comment map[string]interface{}
	err = sonic.Unmarshal(bodyBytes, &comment)
	if err != nil {
		log.Printf("Failed to unmarshal comment response: %v, body: %s", err, string(bodyBytes))
		return nil, fmt.Errorf("failed to unmarshal comment: %w", err)
	}

	log.Printf("Successfully added comment to Jira issue %s: %v", issueKeyOrId, comment)
	return comment, nil
}
