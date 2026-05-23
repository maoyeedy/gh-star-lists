---
name: ascii-check
description: Scan Go source for non-ASCII chars (em-dashes, en-dashes, smart quotes, non-breaking spaces, zero-width spaces) invisible in diffs, break grep/search. Use before commit or suspect copy-paste artifacts. Invoke /ascii-check.
disable-model-invocation: true
---

Run: `LC_ALL=C grep -Pn '[^\x00-\x7F]' --include='*.go' -r .`

If matches found:
- List each `file:line - <offending text>` with Unicode codepoint (e.g. U+2014 EM DASH)
- Suggest ASCII replacement (e.g. `-` → `-`, `"` → `"`, `'` → `'`)

If no matches: report "No non-ASCII characters found."
