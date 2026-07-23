---
description: "Use the Wiz MCP server to find up to three critical severity risk issues for the current repository and present Green Agent investigations and remediation plans"
allowed-tools: ["mcp__Wiz-MCP__discover", "mcp__Wiz-MCP__execute"]
---

# Wiz Remediate

Find up to three critical severity risk issues for the repository `wiz-demo/sorcery-solutions-frontend` in Wiz, then retrieve and present the Green Agent investigation and remediation plan for each.

## Steps

### 1. Look up the repository in Wiz

Call `list_code_repositories` with `search: "sorcery-solutions-frontend"` and `first: 5`. Identify the entry whose `name` is exactly `wiz-demo/sorcery-solutions-frontend` and note its `id` (a UUID).

### 2. Fetch up to three critical issues for that repository

Call `list_issues` with:
- `resource_id`: the UUID from step 1
- `severity`: `["CRITICAL"]`
- `first`: 3

Collect all returned issue IDs.

### 3. For each issue (in parallel where possible)

Call both of the following simultaneously per issue:
- `get_issue` with the issue ID — for full metadata, affected resources, CVEs, service tickets, and remediation strategies
- `get_green_agent_analysis` with the issue ID — for the AI verdict, confidence level, investigation steps, and remediation steps

### 4. Present results

For each issue render a self-contained section using the template below. If fewer than three critical issues exist, present only what was found and note the total at the top. If no critical issues exist, say so clearly.

---

## Output template (repeat per issue)

```
---

## Critical Issue N of <total>: <issue title / source rule name>

**Issue ID:** `<id>`
**Status:** <status> | **Type:** <type> | **Created:** <createdAt date>
**Service Ticket:** <ticket name and URL if present, else "None">

### What Was Found

<2–4 sentence plain-English summary of what the issue is, what resource is affected,
and why it is dangerous. Draw from the source rule description and Green Agent investigation.>

**CVEs involved (if any):**
| CVE | Notes |
|-----|-------|
| CVE-XXXX-XXXXX | <brief note — e.g. pre-auth RCE, CISA KEV> |

**Risk dimensions:** <comma-separated list from sourceRules[].risks>
**Business impact:** <project names and their businessImpact values>
**Green Agent confidence:** <confidenceLevel> — <confidenceReasoning summary>

---

### Green Agent Investigation

<Synthesise all investigationSteps into a single paragraph. Cover: what was found, the full
chain from source to runtime, root cause, and the verdict/owner. Do not use bullet points or
numbered items. Preserve any critical identifiers (file paths, line numbers, CVE IDs, versions)
inline within the paragraph.>

---

### Remediation Plan

**Verdict:** <verdict> | **Summary:** <reasoning content — one or two sentences>

<For each remediationStep, render it as a numbered item with its title bold and content beneath.
Preserve code blocks and diffs exactly as returned.>

**Step 1 — <title>**
<content>

**Step 2 — <title>**
<content>

... and so on

**Assigned owner:** <name and email from investigation, if identified>

---
```

Keep language direct and action-oriented. Do not add commentary or caveats beyond what the Green Agent data contains.
