# Jira Integration Plugin

A Soren plugin for integrating with Jira, allowing you to manage projects, create issues, delete issues, and add comments.

## Structure

This plugin follows a clean, modular structure:

```
jira-plugin/
├── actions/
│   ├── issues/
│   │   ├── actions.go      # Issue-related action definitions
│   │   └── handlers.go     # Issue action handlers
│   └── projects/
│       ├── actions.go      # Project-related action definitions
│       └── handlers.go     # Project action handlers
├── client/
│   └── jira_client.go      # Jira API client implementation
├── credentials/
│   └── credentials.go      # Credentials storage and management
├── handlers.go             # Shared handlers (onboarding, etc.)
├── plugin.go              # Main plugin initialization
├── go.mod                 # Go module definition
└── env.plugin             # Environment configuration
```

## Actions

### Projects
- **projects.list** - List all projects in your Jira instance

### Issues
- **issues.create** - Create a new issue in Jira
- **issues.delete** - Delete an issue by key or ID
- **issues.comment** - Add a comment to an issue

## Features

- **Multi-tenant support**: Each space (entityId) can have its own Jira credentials
- **Dynamic fields**: Support for additional Jira fields through `additionalFields` parameter
- **Synchronous responses**: Quick operations respond directly without async job pattern
- **Error handling**: User-friendly error messages from Jira API responses

## Development

1. Install dependencies:
   ```bash
   go mod tidy
   ```

2. Build the plugin:
   ```bash
   go build -o jira-plugin .
   ```

3. Run the plugin:
   ```bash
   ./jira-plugin
   ```

## Configuration

The plugin requires the following environment variables (set in `env.plugin`):
- `AGENT_URI` - NATS agent URI
- `PLUGIN_ID` - Plugin identifier
- `AGENT_CRED` - NATS credentials
- `SOREN_AUTH_KEY` - Authentication key for event logging
- `SOREN_EVENT_CHANNEL` - NATS channel for events

## Onboarding

Users must complete onboarding by providing:
- Jira Instance URL
- Email address
- API Token

Credentials are stored per space (entityId) for multi-tenant support.
