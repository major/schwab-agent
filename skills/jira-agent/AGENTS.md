# skills/jira-agent agent guide

Generated: Tue Apr 28 2026. Applies to the embedded `jira-agent` LLM skill documentation.

## Purpose

This directory is the AI-facing manual for the `jira-agent` CLI. It helps tool-calling LLMs pick the right Jira command, request bounded output, avoid unsafe writes, and recover from structured errors. Keep it concise, accurate, and source-backed by the live `jira-agent schema` command.

Keep this file, the root `AGENTS.md`, and the skill docs current anytime code changes. Code changes that affect agent-visible behavior must update the relevant `skills/jira-agent` files in the same change.

## Files

| File | Purpose |
| --- | --- |
| `SKILL.md` | Entry point with YAML front matter, auth, global flags, schema discovery, output contracts, exit codes, gotchas. |
| `issues.md` | Issue CRUD, search, bulk operations, metadata discovery. |
| `issue-workflows.md` | Transitions, assignments, comments, worklogs, watchers, votes, attachments, links, changelog, ranking, notifications. |
| `agile.md` | Boards, sprints, epics, backlog, Jira Software Agile API workflows. |
| `project-management.md` | Projects, components, versions. |
| `admin-reference.md` | Fields, users, groups, filters, permissions, dashboards, workflows, statuses, priorities, resolutions, issue types, labels, JQL helpers, server info. |

## Maintenance trigger

Update these docs in the same change when any code change affects these areas:

- Command names, paths, aliases, or default commands.
- Agent-relevant flags, args, required flags, or examples.
- Auth behavior, write-enable behavior, or error remediation.
- Output envelope, error envelope, CSV/TSV behavior, pagination, or exit codes.
- Recommended LLM workflows such as schema discovery, field discovery, and bulk patterns.

## Source of truth

- The live `jira-agent schema` command is authoritative. Do not hand-invent flags or command paths.
- Preferred discovery commands for checking examples:
  - `jira-agent schema --compact`
  - `jira-agent schema --compact --depth 1`
  - `jira-agent schema --category issue --required-only --depth 1`
  - `jira-agent schema --command "issue create" --required-only`
- Examples should prefer canonical paths over aliases. Use `issue bulk <action>` and `issue remote-link`.
- `--pretty` is for humans only. It must never appear in skill files or LLM-facing examples.

## Writing style

- Preserve YAML front matter in `SKILL.md`; external skill loaders consume it.
- Keep docs token-efficient. Use one accurate default example plus only non-obvious flags.
- Prefer workflows over exhaustive flag catalogs because schema provides the catalog.
- Use terse bullets and command examples that an LLM can copy after filling placeholders.
- Explain Jira gotchas that cause failed tool calls: account IDs, transition IDs, ADF JSON, custom field JSON, visibility pairs, write protection, pagination differences.

## Output contract to document

- Default output is JSON. CSV/TSV are for flat tables.
- Success JSON has `data`, optional `errors`, and `metadata`; do not mention a `status` field unless the code adds one.
- Error output is always JSON with `error.code`, `error.message`, and optional `error.details`.
- `schema` emits raw JSON, not the success envelope.
- Exit codes: 0 success, 2 not found, 3 auth, 4 API, 5 validation, 1 unknown.
- Pagination differs by endpoint. `issue search` can use cursor-style `--next-page-token`; most list commands use `--start-at` and `--max-results`.

## Safety guidance

- Writes are blocked unless `JIRA_ALLOW_WRITES=true` or config has `"i-too-like-to-live-dangerously": true`.
- LLM-facing docs must not encourage retries of blocked writes. Tell the user what enablement is missing.
- Read examples should request minimal fields when possible, for example `--fields key,summary,status`.
- Mutation examples should show discovery first when Jira configuration affects required fields, especially `issue meta` before issue create/edit.
