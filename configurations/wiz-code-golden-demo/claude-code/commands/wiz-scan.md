---
description: "Run a Wiz SAST scan on the current directory and present findings grouped by severity"
allowed-tools: ["Bash"]
---

# Wiz SAST Scan

Run a Wiz CLI SAST scan on the current directory and present the findings.

## Steps

1. Run the scan and extract SAST results using this exact command — do not modify it (no `2>&1`, no `tee`, no additional flags):

```bash
wizcli scan dir . --use-device-code --no-publish --by-policy-hits=DISABLED --disabled-scanners=Vulnerability,Secret,Misconfiguration,SensitiveData,Malware --stdout json | jq '.result.sast'
```

2. Parse the JSON output. The `result.sast` array contains finding objects. Each finding has at minimum:
   - `severity` (e.g. `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFORMATIONAL`)
   - `title` or `ruleId` — the name/ID of the rule that triggered
   - `filePath` — source file where the issue was found
   - `line` (or `lineNumber`) — line number
   - `description` or `message` — explanation of the finding
   - Additional fields such as `remediation` if present

3. Group findings by severity in this order: CRITICAL → HIGH → MEDIUM → LOW → INFORMATIONAL. Within each group list findings as a numbered sub-list.

4. Present output using this structure:

```
## Wiz SAST Scan Results

**X findings total**

### CRITICAL (N)
1. **<title/ruleId>** — `<filePath>:<line>`
   <description>

### HIGH (N)
...

### MEDIUM (N)
...

### LOW (N)
...

### INFORMATIONAL (N)
...
```

If the `result.sast` array is empty or null, report: "No SAST findings detected."

If the command fails (non-zero exit), surface the error output to the user and stop.
