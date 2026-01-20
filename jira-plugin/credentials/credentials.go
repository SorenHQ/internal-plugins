package credentials

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const credentialsFileName = "jira_credentials.json"

// JiraCredentials represents the stored Jira credentials
type JiraCredentials struct {
	InstanceURL string `json:"instanceUrl"`
	Email       string `json:"email"`
	APIToken    string `json:"apiToken"`
}

// CredentialsStorage handles storing and retrieving credentials
type CredentialsStorage struct {
	filePath string
}

var globalCredentialsStorage *CredentialsStorage

// GetCredentialsStorage returns the global credentials storage instance
func GetCredentialsStorage() *CredentialsStorage {
	if globalCredentialsStorage == nil {
		globalCredentialsStorage = NewCredentialsStorage()
	}
	return globalCredentialsStorage
}

// NewCredentialsStorage creates a new credentials storage instance
func NewCredentialsStorage() *CredentialsStorage {
	// Store credentials in the same directory as the plugin binary
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}
	return &CredentialsStorage{
		filePath: filepath.Join(dir, credentialsFileName),
	}
}

// SaveCredentials saves credentials to file using spaceID as the key
func (cs *CredentialsStorage) SaveCredentials(spaceID string, creds JiraCredentials) error {
	// Read existing credentials if file exists
	allCreds, err := cs.loadAllCredentials()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load existing credentials: %w", err)
	}

	// Use spaceID as the key; if empty, use "default"
	spaceKey := spaceID
	if spaceKey == "" {
		spaceKey = "default"
	}

	if allCreds == nil {
		allCreds = make(map[string]JiraCredentials)
	}

	// Store credentials for this space (spaceID is the key, not stored in the struct)
	allCreds[spaceKey] = creds

	// Write back to file
	data, err := json.MarshalIndent(allCreds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	err = os.WriteFile(cs.filePath, data, 0600) // 0600 = read/write for owner only
	if err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// GetCredentials retrieves credentials for a specific space
// If spaceID is empty, returns default credentials
func (cs *CredentialsStorage) GetCredentials(spaceID string) (*JiraCredentials, error) {
	allCreds, err := cs.loadAllCredentials()
	if err != nil {
		return nil, err
	}

	spaceKey := spaceID
	if spaceKey == "" {
		spaceKey = "default"
	}

	creds, exists := allCreds[spaceKey]
	if !exists {
		return nil, fmt.Errorf("credentials not found for space: %s", spaceKey)
	}

	return &creds, nil
}

// HasCredentials checks if credentials exist for a specific space
func (cs *CredentialsStorage) HasCredentials(spaceID string) bool {
	creds, err := cs.GetCredentials(spaceID)
	return err == nil && creds != nil
}

// loadAllCredentials loads all credentials from file
func (cs *CredentialsStorage) loadAllCredentials() (map[string]JiraCredentials, error) {
	data, err := os.ReadFile(cs.filePath)
	if err != nil {
		return nil, err
	}

	var allCreds map[string]JiraCredentials
	err = json.Unmarshal(data, &allCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return allCreds, nil
}

// GetAllSpaces returns a list of all space IDs that have credentials
func (cs *CredentialsStorage) GetAllSpaces() ([]string, error) {
	allCreds, err := cs.loadAllCredentials()
	if err != nil {
		return []string{}, err
	}

	spaces := make([]string, 0, len(allCreds))
	for spaceID := range allCreds {
		spaces = append(spaces, spaceID)
	}

	return spaces, nil
}
