# Admin Reference

Field management, users, groups, filters, permissions, dashboards, workflows, statuses, priorities, resolutions, issue types, labels, JQL helpers, and server info.

## Fields

### field list / search

```bash
jira-agent field list
jira-agent field search -q "story points" --type custom
jira-agent field search -q "priority" --type system --max-results 10
```

| Flag | Notes |
|------|-------|
| `-q` / `--query` | Search term |
| `--type` | `custom` or `system` |
| `--max-results` | Default 50 |
| `--start-at` | Offset |

### field context list / create / update / delete

```bash
jira-agent field context list customfield_10001
jira-agent field context create customfield_10001 --name "Project X Context" \
  --projects PROJ1,PROJ2 --issue-types Story,Bug
jira-agent field context update customfield_10001 10500 --name "Renamed"
jira-agent field context delete customfield_10001 10500
```

| Flag | Notes |
|------|-------|
| `--name` | Required for create |
| `--description` | Context description |
| `--projects` | Comma-separated project IDs (create only) |
| `--issue-types` | Comma-separated issue type IDs (create only) |
| `--max-results` | Page size (list, default 50) |
| `--start-at` | Offset (list) |

### field option list / create / update / delete / reorder

```bash
jira-agent field option list customfield_10001 10500
jira-agent field option create customfield_10001 10500 --values "Option A,Option B"
jira-agent field option update customfield_10001 10500 --option-id 10600 --value "Renamed" --disabled
jira-agent field option delete customfield_10001 10500 10600
jira-agent field option reorder customfield_10001 10500 --option-ids 10600,10601 \
  --position After --anchor 10602
```

| Flag | Notes |
|------|-------|
| `--values` | Comma-separated (create only) |
| `--option-id` | Option to update (update only) |
| `--value` | New value (update only) |
| `--disabled` | Disable option (update only) |
| `--option-ids` | IDs to move (reorder only) |
| `--position` | `First`, `Last`, `Before`, `After` (case-sensitive, reorder only) |
| `--anchor` | Required for Before/After positions (reorder only) |

## Users

### user get / search / groups

```bash
jira-agent user get --account-id 5b10ac8d82e05b22cc7d4ef5
jira-agent user get --account-id 5b10ac8d --expand groups,applicationRoles
jira-agent user search --query "john"
jira-agent user search --query "john" --max-results 10
jira-agent user groups --account-id 5b10ac8d82e05b22cc7d4ef5
```

| Flag | Notes |
|------|-------|
| `--account-id` | Required for get/groups |
| `--query` | Search by name/email (search only) |
| `--expand` | e.g., `groups,applicationRoles` |
| `--max-results` | Default 50 (search only) |
| `--start-at` | Offset (search only) |

## Groups

### group list / get

```bash
jira-agent group list
jira-agent group list --query "dev" --case-insensitive
jira-agent group get --groupname "jira-software-users" --expand users
jira-agent group get --group-id abc123-def456
```

### group create / delete

```bash
jira-agent group create "new-team"
jira-agent group delete --groupname "old-team"
jira-agent group delete --group-id abc123 --swap-group "replacement-team"
```

`--swap-group`/`--swap-group-id` reassigns permissions before deletion.

### group members

```bash
jira-agent group members --groupname "developers" --max-results 50
jira-agent group members --group-id abc123 --include-inactive
```

### group add-member / remove-member

```bash
jira-agent group add-member --account-id 5b10ac8d --groupname "developers"
jira-agent group remove-member --account-id 5b10ac8d --group-id abc123
```

`--account-id` required. Identify group by `--groupname` or `--group-id`.

### group member-picker

```bash
jira-agent group member-picker --query "john"
jira-agent group member-picker --query "j" --issue-key PROJ-100 --project-id 10000
```

Autocomplete for group member assignment. Filters by `--query`, scoped by `--issue-key`/`--project-id`.

## Filters

### filter list / get / favorites

```bash
jira-agent filter list --name "Sprint" --expand jql
jira-agent filter list --account-id 5b10ac8d --project-id 10000
jira-agent filter get 10500 --expand jql,subscriptions
jira-agent filter favorites
```

### filter create / update / delete

```bash
jira-agent filter create --name "My Bugs" --jql "assignee = currentUser() AND type = Bug"
jira-agent filter create --name "Sprint Board" --jql "sprint in openSprints()" \
  --description "Active sprint filter"
jira-agent filter update 10500 --name "Renamed" --jql "updated query"
jira-agent filter delete 10500
```

| Flag | Notes |
|------|-------|
| `--name` | Filter name |
| `--jql` | JQL query |
| `--description` | Filter description |
| `--body-json` | Full JSON body (advanced) |
| `--expand` | e.g., `jql,subscriptions` |
| `--override-share-permissions` | Boolean |

### filter permissions / share / unshare / default-share-scope

```bash
jira-agent filter permissions 10500
jira-agent filter share 10500 --with user:5b10ac8d82e05b22cc7d4ef5
jira-agent filter unshare 10500 --permission-id 10200
jira-agent filter default-share-scope get
jira-agent filter default-share-scope set --scope PRIVATE
```

## Permissions

### permission check / list

```bash
jira-agent permission check --permissions BROWSE_PROJECTS,CREATE_ISSUES --project-key PROJ
jira-agent permission check --permissions EDIT_ISSUES --issue-key PROJ-100
jira-agent permission list
```

| Flag | Notes |
|------|-------|
| `--permissions` | Required, comma-separated permission keys |
| `--project-key` | Scope to project |
| `--project-id` | Alternative to project-key |
| `--issue-key` | Scope to issue |
| `--issue-id` | Alternative to issue-key |
| `--comment-id` | Scope to comment |

### permission schemes list / get / project

```bash
jira-agent permission schemes list
jira-agent permission schemes get 10000
jira-agent permission schemes project PROJ
```

## Dashboards

### dashboard list / get / gadgets

```bash
jira-agent dashboard list
jira-agent dashboard list --filter my --max-results 10
jira-agent dashboard get 10100
jira-agent dashboard gadgets 10100
jira-agent dashboard gadgets 10100 --module-key "com.atlassian.jira.gadgets:filter-results-gadget"
```

| Flag | Notes |
|------|-------|
| `--filter` | `my` or `favorite` (list only) |
| `--gadget-id` | Filter gadgets (gadgets only) |
| `--module-key` | Filter by module (gadgets only) |
| `--uri` | Filter by URI (gadgets only) |

### dashboard create / copy / update / delete

```bash
jira-agent dashboard create --name "Team dashboard"
jira-agent dashboard copy 10100 --name "Team dashboard copy"
jira-agent dashboard update 10100 --name "Renamed"
jira-agent dashboard delete 10100
```

## Workflows

### workflow list / get

```bash
jira-agent workflow list
jira-agent workflow list --query "software" --is-active --scope global
jira-agent workflow get abc123-workflow-id
jira-agent workflow get abc123 --name "Software Simplified Workflow"
```

| Flag | Notes |
|------|-------|
| `--query` | Search term (list) |
| `--scope` | Filter scope (list) |
| `--is-active` | Active only (list) |
| `--order-by` | Sort (list) |
| `--expand` | Expansions |
| `--name` | Workflow name (get, alternative to ID) |

### workflow statuses

```bash
jira-agent workflow statuses
```

Lists all statuses across workflows.

### workflow scheme list / get / project

```bash
jira-agent workflow scheme list
jira-agent workflow scheme get 10000
jira-agent workflow scheme project PROJ
```

### workflow transition-rules

```bash
jira-agent workflow transition-rules --workflow-id abc123
jira-agent workflow transition-rules --project-id 10000 --issue-type-id 10001
```

## Statuses

```bash
jira-agent status list
jira-agent status get "In Progress"
jira-agent status get 10001
jira-agent status categories
```

`get` accepts status name or ID. `categories` lists status categories (To Do, In Progress, Done).

## Priorities / Resolutions / Labels

```bash
jira-agent priority list
jira-agent resolution list
jira-agent label list
```

Read-only lists, no flags.

## Issue Types

```bash
jira-agent issuetype list
jira-agent issuetype get 10001
jira-agent issuetype project 10000
```

Alias: `issue-type`. `project` lists types available in a specific project.

## JQL Helpers

### jql fields / suggest / validate

```bash
jira-agent jql fields
jira-agent jql suggest --field-name status --field-value "In"
jira-agent jql suggest --field-name assignee --predicate-name "was"
jira-agent jql validate --query "project = PROJ AND status = 'Done'"
jira-agent jql validate --query "bad query" --validation warn
```

| Command | Key Flags |
|---------|-----------|
| `fields` | None. Lists JQL-searchable fields |
| `suggest` | `--field-name` (required), `--field-value`, `--predicate-name`, `--predicate-value` |
| `validate` | `--query` (required, repeatable), `--validation` (`strict`/`warn`/`none`, default `strict`) |

## Server Info

```bash
jira-agent server-info
```

Returns Jira instance details (version, deployment type, URLs). No flags.

## whoami

```bash
jira-agent whoami
```

Verify auth. Returns current user info. No flags.
