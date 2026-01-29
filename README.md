# Internal Plugins

This repository contains internal Soren plugins and a sample plugin
(`jira-plugin`) you can use as a starting point for development and testing.

## Plugins and Actions

| Plugin | Status | Implemented Actions | Planned Actions |
| --- | --- | --- | --- |
| Jira | Sample | `projects.list`, `issues.create`, `issues.delete`, `issues.comment` | `issues.update`, `issues.transition`, `issues.get`, `projects.get` |
| Google Calendar | Planned | — | `calendars.list`, `events.list`, `events.create`, `events.update`, `events.delete` |
| Slack | Planned | — | `channels.list`, `messages.post`, `messages.update`, `messages.delete`, `users.list` |

## Notes

- The `jira-plugin` directory is intended as a reference implementation you can
  copy or extend for new internal plugins.
