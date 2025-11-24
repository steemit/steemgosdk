# Cursor Rules

## Language Rules

- **User Interaction**: Use Chinese (中文) when communicating with the user
- **Code Comments**: Use English
- **Commit Messages**: Use English
- **Documentation**: Use English
- **Rules and Configuration**: Use English
- **Code**: Use English (variable names, function names, etc.)

## Git Operations Rules

- **All git operations must be non-interactive**: Use environment variables, scripts, or command-line flags to avoid interactive prompts
- **Examples**:
  - Use `GIT_EDITOR`, `GIT_SEQUENCE_EDITOR` environment variables for rebase operations
  - Use `--no-edit` flag for commit amendments when appropriate
  - Use scripts with `sed` or other tools to modify git todo files
  - Never use interactive editors like `vim` or `nano` in git operations
  - Use `git rebase` with prepared scripts instead of interactive mode

## Summary

All content except user interaction should be in English. Only when directly communicating with the user should Chinese be used.

All git operations must be performed non-interactively using scripts, environment variables, or command-line flags.

