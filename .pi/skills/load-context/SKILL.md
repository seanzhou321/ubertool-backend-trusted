---
name: "load-context"
description: "Load and display a previous pi session from the session store. Lists available sessions and lets you pick one to view."
argument-hint: "Optional session ID or date filter (e.g., '2026-08-07' or full session ID)"
compatibility: "Requires pi agent environment with session files in ./.pi/context/"
metadata:
  author: "pi-user"
user-invocable: true
disable-model-invocation: false
---

# Load Context Skill

Load and view previous pi agent sessions stored in `./.pi/context/` (project-relative).

## Usage

```bash
/skill:load-context                    # Interactive: list all sessions and pick one
/skill:load-context 2026-08-07         # Filter by date prefix
/skill:load-context 019fda32-d831      # Filter by session ID prefix
/skill:load-context --list             # Just list sessions without loading
/skill:load-context --latest           # Load the most recent session
```

## How It Works

1. Discovers session files under `./.pi/context/`
2. Lists all `.jsonl` session files with timestamps and IDs
3. Lets you select a session (or filters by argument)
4. Parses and displays the session in a readable format:
   - Session metadata (ID, timestamp, working directory)
   - User messages
   - Assistant responses (including tool calls)
   - Tool results
   - Thinking/reasoning blocks

## Session File Format

Sessions are stored as JSONL (newline-delimited JSON) with message types:
- `session` - Session metadata
- `message` - User/assistant messages with tool calls
- `model_change` - Model/provider changes
- `thinking_level_change` - Thinking level adjustments

## Example Output

```
=== Session: 019fda32-d831-7533-87ba-639cfc0bb816 ===
Date: 2026-08-07T03:09:53.073Z
CWD: C:\Users\yixio\ubertool\ubertool-backend-trusted
Model: nvidia/nemotron-3-ultra-550b-a55b:free (openrouter)

--- User (2026-08-07T03:13:01) ---
Please create a pi skill "load-session"...

--- Assistant (2026-08-07T03:13:03) ---
[Thinking] The user wants me to create a pi skill...
[Tool: bash] ls -la ~/.pi/agent/sessions/
[Tool Result] Session files found...
...
```

## Implementation

The skill uses a helper script `scripts/load-context.sh` to:
1. Find all session files under `./.pi/context/`
2. Sort by timestamp (newest first)
3. Filter by user-provided argument
4. Display formatted session content