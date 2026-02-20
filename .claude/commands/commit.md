---
name: commit
description: Run quality checks, then commit and push with a Conventional Commits message
---

1. Run quality gate:
   ```bash
   make check
   ```
   Fix ALL errors before continuing.

2. Review what changed:
   ```bash
   git status
   git diff
   ```

3. Stage and commit using Conventional Commits format:
   - `feat:` new command or behaviour
   - `fix:` bug fix
   - `docs:` documentation only
   - `chore:` maintenance, deps
   - `refactor:` restructure, no behaviour change
   - `ci:` workflow changes
   - Imperative, lowercase, no trailing period

   ```bash
   git add <files>
   git commit -m "type: description"
   git push
   ```
