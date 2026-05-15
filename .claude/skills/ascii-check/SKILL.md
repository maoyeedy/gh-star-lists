---
name: ascii-check
description: Scan Go source files for non-ASCII characters (em-dashes, en-dashes, smart quotes, non-breaking spaces, zero-width spaces) that are invisible in diffs and silently break grep/search. Use before committing or when suspecting copy-paste artifacts. Invoke with /ascii-check.
disable-model-invocation: true
---

Run: `LC_ALL=C grep -Pn '[^\x00-\x7F]' --include='*.go' -r .`

If matches found:
- List each as `file:line — <offending text>` with the Unicode codepoint (e.g. U+2014 EM DASH)
- Suggest the ASCII replacement (e.g. `—` → `-`, `"` → `"`, `'` → `'`)

If no matches: report "No non-ASCII characters found."
