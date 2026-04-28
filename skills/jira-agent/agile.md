# Agile

Board, sprint, epic, and backlog management. Uses Jira Software Agile API (`/rest/agile/1.0`). Requires Jira Software access.

## Boards

### board list / get

```bash
jira-agent board list
jira-agent board list --project PROJ --type scrum --name "Platform" --max-results 25
jira-agent board get 84
```

| Flag | Notes |
|------|-------|
| `--type` | `scrum` or `kanban` (list only) |
| `--name` | Filter by name (list only) |
| `--project` | Project key or ID referenced by board filter (list only) |
| `--max-results` | Default 50 (list only) |
| `--start-at` | Offset (list only) |

### board config

```bash
jira-agent board config 84
```

Returns board configuration (columns, estimation, ranking). Args: `<board-id>`.

### board epics

```bash
jira-agent board epics 84
jira-agent board epics 84 --done --max-results 10
```

| Flag | Notes |
|------|-------|
| `--done` | Include completed epics |
| `--max-results` | Default 50 |
| `--start-at` | Offset |

### board issues

```bash
jira-agent board issues 84 --fields key,summary,status
jira-agent board issues 84 --jql "assignee = currentUser()" --expand changelog
```

| Flag | Notes |
|------|-------|
| `--fields` | Comma-separated |
| `--jql` | Filter issues |
| `--expand` | Expansions |
| `--max-results` | Default 50 |
| `--start-at` | Offset |

### board projects / versions

```bash
jira-agent board projects 84
jira-agent board versions 84 --max-results 20
```

Pagination: `--max-results`, `--start-at`. Args: `<board-id>`.

### board create / filter / delete / property

```bash
jira-agent board create --name "Team board" --type scrum --filter 10000
jira-agent board create --name "Team board" --type kanban --filter 10000 --location-project PROJ
jira-agent board filter 10000 --max-results 25
jira-agent board delete 84
jira-agent board property list 84
jira-agent board property set 84 com.example.flag --value-json '{"enabled":true}'
```

`property` also supports `get` and `delete` with args `<board-id> <property-key>`.

## Sprints

### sprint list

```bash
jira-agent sprint list --board-id 84
jira-agent sprint list --board-id 84 --state active
jira-agent sprint list --board-id 84 --state future,active --max-results 5
```

| Flag | Notes |
|------|-------|
| `--board-id` | Required |
| `--state` | `future`, `active`, `closed` (comma-sep for multiple) |
| `--max-results` | Default 50 |
| `--start-at` | Offset |

### sprint get

```bash
jira-agent sprint get 42
```

### sprint create

```bash
jira-agent sprint create --board-id 84 --name "Sprint 12"
jira-agent sprint create --board-id 84 --name "Sprint 12" \
  --start-date 2026-05-01T00:00:00.000Z --end-date 2026-05-14T00:00:00.000Z \
  --goal "Complete auth module"
```

| Flag | Notes |
|------|-------|
| `--board-id` | Required |
| `--name` | Required |
| `--start-date` | ISO 8601 |
| `--end-date` | ISO 8601 |
| `--goal` | Sprint goal text |

### sprint update

```bash
jira-agent sprint update 42 --name "Sprint 12 (Extended)"
jira-agent sprint update 42 --state closed
jira-agent sprint update 42 --goal "Revised goal" --end-date 2026-05-21T00:00:00.000Z
```

Same optional flags as create plus `--state`. Args: `<sprint-id>`.

### sprint delete

```bash
jira-agent sprint delete 42
```

### sprint issues

```bash
jira-agent sprint issues 42 --fields key,summary,status,assignee
jira-agent sprint issues 42 --jql "status != Done" --max-results 100
```

| Flag | Notes |
|------|-------|
| `--fields` | Comma-separated |
| `--jql` | Filter within sprint |
| `--expand` | Expansions |
| `--max-results` | Default 50 |
| `--start-at` | Offset |

### sprint move-issues

```bash
jira-agent sprint move-issues 42 --issues KEY-1,KEY-2,KEY-3
jira-agent sprint move-issues 42 --issues KEY-1 --rank-before KEY-5
jira-agent sprint move-issues 42 --issues KEY-1 --rank-after KEY-10
```

| Flag | Notes |
|------|-------|
| `--issues` | Required, comma-separated |
| `--rank-before` | Position before this issue |
| `--rank-after` | Position after this issue |

### sprint swap / property

```bash
jira-agent sprint swap 100 101
jira-agent sprint property list 42
jira-agent sprint property get 42 com.example.flag
jira-agent sprint property set 42 com.example.flag --value-json '{"enabled":true}'
jira-agent sprint property delete 42 com.example.flag
```

## Epics

### epic get

```bash
jira-agent epic get PROJ-100
jira-agent epic get 10042
```

Args: `<epic-id-or-key>`.

### epic issues

```bash
jira-agent epic issues PROJ-100 --fields key,summary,status
jira-agent epic issues PROJ-100 --jql "status = 'In Progress'" --max-results 20
```

| Flag | Notes |
|------|-------|
| `--fields` | Comma-separated |
| `--jql` | Filter within epic |
| `--max-results` | Default 50 |
| `--start-at` | Offset |

### epic move-issues

```bash
jira-agent epic move-issues PROJ-100 --issues KEY-1,KEY-2
```

`--issues` required. Moves issues into the epic.

### epic orphans

```bash
jira-agent epic orphans --fields key,summary
jira-agent epic orphans --jql "project = PROJ" --fields key,summary,status
```

Lists issues with no epic. Use `--jql` to scope by project or other criteria. Same pagination flags as `epic issues`.

### epic remove-issues

```bash
jira-agent epic remove-issues --issues KEY-1,KEY-2
```

`--issues` required. Removes issues from their epic (issues become orphans).

### epic rank

```bash
jira-agent epic rank PROJ-100 --rank-before PROJ-200
jira-agent epic rank PROJ-100 --rank-after PROJ-50
```

| Flag | Notes |
|------|-------|
| `--rank-before` | Rank before this epic |
| `--rank-after` | Rank after this epic |

## Backlog

### backlog list

```bash
jira-agent backlog list --board-id 84
jira-agent backlog list --board-id 84 --fields key,summary,status --jql "type = Bug"
```

| Flag | Notes |
|------|-------|
| `--board-id` | Required |
| `--fields` | Comma-separated |
| `--jql` | Filter backlog items |
| `--max-results` | Default 50 |
| `--start-at` | Offset |

### backlog move

```bash
jira-agent backlog move --issues KEY-1,KEY-2
```

`--issues` required. Moves issues to the backlog (removes from any sprint).

## Workflows

### Sprint planning

```bash
# Find the board
jira-agent board list --project PROJ --type scrum

# List sprints, find active
jira-agent sprint list --board-id 84 --state active

# Check sprint contents
jira-agent sprint issues 42 --fields key,summary,status,assignee --output csv

# Move items into sprint
jira-agent sprint move-issues 42 --issues PROJ-10,PROJ-11

# Move items to backlog
jira-agent backlog move --issues PROJ-15
```

### Epic management

```bash
# Check epic progress
jira-agent epic issues PROJ-100 --fields key,summary,status --output csv

# Find issues without an epic
jira-agent epic orphans --jql "project = PROJ" --fields key,summary

# Move issues into epic
jira-agent epic move-issues PROJ-100 --issues PROJ-50,PROJ-51
```
