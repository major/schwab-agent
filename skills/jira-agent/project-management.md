# Project Management

Projects, components, and versions.

## Projects

### project list / get

```bash
jira-agent project list
jira-agent project list --query "backend" --type-key software --max-results 20 --output csv
jira-agent project get PROJ
jira-agent project get PROJ --expand description,issueTypes,lead
```

| Flag | Notes |
|------|-------|
| `--query` / `-q` | Filter by name/key (list only) |
| `--type-key` | `business`, `service_desk`, `software` (list only) |
| `--order-by` | `category`, `key`, `name`, `owner`; prefix `-` for desc (list only) |
| `--expand` | list: `description,lead,issueTypes,url,insight`; get: `description,issueTypes,lead,projectKeys` |
| `--max-results` | Max 100 (list only, default 50) |
| `--start-at` | Offset (list only) |

### project roles / categories

```bash
jira-agent project roles list PROJ
jira-agent project roles get PROJ 10000 --exclude-inactive-users
jira-agent project categories list
jira-agent project categories get 10000
```

## Components

### component list

```bash
jira-agent component list --project PROJ
jira-agent component list --project PROJ --query "auth" --order-by name
```

| Flag | Notes |
|------|-------|
| `--project` | Required |
| `--query` | Filter by name |
| `--order-by` | Sort field |
| `--max-results` | Default 50 |
| `--start-at` | Offset |

### component get / issue-counts

```bash
jira-agent component get 10500
jira-agent component issue-counts 10500
```

Args: `<component-id>`.

### component create

```bash
jira-agent component create --project PROJ --name "Authentication"
jira-agent component create --project PROJ --name "Auth" \
  --description "Auth module" --lead-account-id 5b10ac8d --assignee-type PROJECT_LEAD
```

| Flag | Notes |
|------|-------|
| `--project` | Required |
| `--name` | Required |
| `--description` | Component description |
| `--lead-account-id` | Component lead |
| `--assignee-type` | `PROJECT_DEFAULT`, `COMPONENT_LEAD`, `PROJECT_LEAD`, `UNASSIGNED` |

### component update

```bash
jira-agent component update 10500 --name "Renamed" --description "Updated desc"
```

Same optional flags as create. Args: `<component-id>`.

### component delete

```bash
jira-agent component delete 10500
jira-agent component delete 10500 --move-issues-to 10501
```

`--move-issues-to` reassigns issues to another component before deletion.

## Versions

### version list

```bash
jira-agent version list --project PROJ
jira-agent version list --project PROJ --status released --query "v2" --order-by -releaseDate
```

| Flag | Notes |
|------|-------|
| `--project` | Required |
| `--status` | Filter: `released`, `unreleased`, etc. |
| `--query` | Filter by name |
| `--order-by` | Sort field; prefix `-` for desc |
| `--expand` | Expansions |
| `--max-results` | Default 50 |
| `--start-at` | Offset |

### version get

```bash
jira-agent version get 10100
jira-agent version get 10100 --expand issuesstatus
```

### version create

```bash
jira-agent version create --project PROJ --name "v2.1.0"
jira-agent version create --project PROJ --name "v2.1.0" \
  --description "Bug fixes" --start-date 2026-05-01 --release-date 2026-06-01
```

| Flag | Notes |
|------|-------|
| `--project` | Required |
| `--name` | Required |
| `--description` | Version description |
| `--start-date` | YYYY-MM-DD |
| `--release-date` | YYYY-MM-DD |
| `--released` | Mark as released |
| `--archived` | Mark as archived |

### version update

```bash
jira-agent version update 10100 --name "v2.1.1" --released
jira-agent version update 10100 --release-date 2026-06-15 --archived=false
```

Same optional flags as create. Args: `<version-id>`.

### version delete

```bash
jira-agent version delete 10100
jira-agent version delete 10100 --move-affected-issues-to 10101 --move-fix-issues-to 10101
```

| Flag | Notes |
|------|-------|
| `--move-affected-issues-to` | Version ID for affected issues |
| `--move-fix-issues-to` | Version ID for fix issues |

### version merge

```bash
jira-agent version merge 10100 10101
```

Merges source into target. Args: `<source-version-id> <target-version-id>`.

### version move

```bash
jira-agent version move 10100 --position First
jira-agent version move 10100 --after 10099
```

| Flag | Notes |
|------|-------|
| `--position` | `Earlier`, `Later`, `First`, `Last` |
| `--after` | Position after this version ID |

### version issue-counts / unresolved-count

```bash
jira-agent version issue-counts 10100
jira-agent version unresolved-count 10100
```

Args: `<version-id>`.

## Workflows

### Release planning

```bash
# List versions
jira-agent version list --project PROJ --status unreleased --output csv

# Check what's left
jira-agent version unresolved-count 10100

# Release it
jira-agent version update 10100 --released --release-date 2026-04-27

# Merge abandoned version into current
jira-agent version merge 10099 10100
```

### Component management

```bash
# Audit components
jira-agent component list --project PROJ --output csv
jira-agent component issue-counts 10500

# Clean up: move issues and delete
jira-agent component delete 10500 --move-issues-to 10501
```
