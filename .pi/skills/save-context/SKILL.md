---
name: "save-context"
description: "Summarize the current context's important information and save it to disk for later restoration."
argument-hint: "Optional focus areas for the summary"
compatibility: "Requires pi agent environment"
metadata:
  author: "pi-user"
user-invocable: true
disable-model-invocation: false
---

## Goal

Summarize everything important from this session, including:
- goals
- plans
- specs
- tool results
- code
- diffs
- errors
- instructions
- unresolved tasks

## Execution

Save the summary to `./.pi/context/context-summary.md` for later restoration.