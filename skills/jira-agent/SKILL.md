---
name: jira-agent
description: "Jira Cloud CLI for AI agents. Structured JSON/CSV/TSV output, semantic exit codes. Covers: issue CRUD, search, bulk ops (create/fetch/delete/edit/move/transition), transitions, assignments, comments, worklogs, watchers, votes, attachments, issue links, remote links, changelog, ranking, notifications; agile boards, sprints, epics, backlogs; projects, components, versions; fields (contexts/options), users, groups, filters, permissions, dashboards, workflows, statuses, priorities, resolutions, issue types, labels, JQL helpers, and server info. Triggers: 'jira', 'jira issue', 'jira search', 'jql', 'jira create', 'jira bulk', 'jira transition', 'jira assign', 'jira comment', 'jira worklog', 'jira sprint', 'jira epic', 'jira board', 'jira backlog', 'jira component', 'jira version', 'jira project', 'jira field', 'jira user', 'jira group', 'jira filter', 'jira permission', 'jira dashboard', 'jira workflow', 'jira status', 'jira-agent'."
metadata:
  author: "Major Hayden"
  version: "2.0.0"
---

# jira-agent CLI

Go CLI for Jira Cloud REST API v3. All output is structured, errors always JSON, exit codes are semantic.

## Feedback

If you hit bugs, confusing usability, missing guidance, or token-inefficient workflows while using `jira-agent`, encourage the user to open a GitHub issue at `github.com/major/jira-agent`. Offer to open the issue for them with GitHub's `gh` CLI if it is installed and the user wants you to file it.

## Companion Files

This is the entry point. Command details are split by theme:

| File | Scope |
|------|-------|
| [issues.md](issues.md) | Issue CRUD, search, bulk ops, meta, count |
| [issue-workflows.md](issue-workflows.md) | Transition, assign, comment, worklog, watcher, vote, attachment, link, remote-link, changelog, rank, notify |
| [agile.md](agile.md) | Board, sprint, epic, backlog |
| [project-management.md](project-management.md) | Project, component, version |
| [admin-reference.md](admin-reference.md) | Field, user, group, filter, permission, dashboard, workflow, status, priority, resolution, issuetype, label, JQL, server-info |

## Auth

Env vars (override config file):

```bash
export JIRA_INSTANCE="your-domain.atlassian.net"
export JIRA_EMAIL="you@example.com"
export JIRA_API_KEY="your-api-token"
export JIRA_ALLOW_WRITES=true  # optional: enable write operations
```

Config file fallback: `~/.config/jira-agent/config.json` (XDG_CONFIG_HOME aware). Verify with `jira-agent whoami`.

### Write Protection

Writes (create, edit, delete, transition, assign) are disabled by default. Enable:

- Config: `"i-too-like-to-live-dangerously": true`
- Env: `JIRA_ALLOW_WRITES=true`

Blocked writes return exit 5 with remediation. Read-only commands always work. `issue transition --list` is read-only.

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--project` | `-p` | | Default Jira project key (also `JIRA_PROJECT` env) |
| `--output` | `-o` | `json` | `json`, `csv`, or `tsv` |
| `--verbose` | `-v` | off | Verbose logging to stderr |
| `--config` | | | Config file path override |

## Schema Discovery

Use schema before guessing flags. Start compact, drill into what you need:

```bash
jira-agent schema --compact                          # command index for routing
jira-agent schema --category issue --required-only   # one command family
jira-agent schema --command "issue create"            # full details for one command
jira-agent schema --command "issue link"              # subtree: returns link + all leaves
```

Parent command filters return descendant commands, so `--command "issue link"` covers `list`, `add`, `delete`, `types`.

For reads/searches, request only needed fields: `--fields key,summary,status`. Use CSV/TSV for simple tables, JSON for updates or nested data.

## Output

### Success envelope (JSON)

```json
{
  "data": { ... },
  "errors": [],
  "metadata": { "timestamp": "...", "total": 42, "returned": 10, "start_at": 0, "max_results": 50 }
}
```

Access results via `.data`. Check `.metadata` for pagination.

### Error response (always JSON, regardless of --output)

```json
{
  "error": { "code": "NOT_FOUND", "message": "Issue KEY-999 not found", "details": "..." }
}
```

| Code | Exit | Meaning |
|------|------|---------|
| `AUTH_FAILED` | 3 | Missing or invalid credentials |
| `NOT_FOUND` | 2 | Resource does not exist |
| `API_ERROR` | 4 | Jira API error |
| `VALIDATION_ERROR` | 5 | Invalid input or blocked write |
| `UNKNOWN` | 1 | Unexpected error |

### CSV/TSV

Flat rows with header, no envelope. Nested values become inline JSON in cells.

## Gotchas

- **Description**: Plain text auto-converts to ADF. Pass valid ADF JSON for structured content.
- **Custom fields**: `--field key=value` parses value as JSON if valid, else raw string. Quote carefully in shell.
- **Project flag**: Command-level `--project` overrides root `-p`.
- **Type resolution**: Issue type matching is case-insensitive.
- **Transition resolution**: Matches status/transition name (case-insensitive), not numeric ID.
- **Schema output**: Raw JSON, not wrapped in success envelope.
- **Pagination**: `issue search` uses `--next-page-token` (cursor). Most other list commands use `--start-at` (offset).
- **Assignment**: `issue assign` accepts account ID, not email. `--unassign` clears, `--default` uses project default.
- **Visibility**: Both `--visibility-type` and `--visibility-value` must be set together.
- **Write protection**: All writes blocked unless `JIRA_ALLOW_WRITES=true` or config set. Exit 5 with remediation.
