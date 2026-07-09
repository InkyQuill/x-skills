# Align remote install command with npx skills add

Go remote installs use a top-level `add SOURCE [SKILL_NAME...]` command as the primary user-facing flow. This keeps x-skills aligned with the upstream `npx skills add` mental model while preserving x-skills behavior: added skills are archived first and linked to selected active roots unless `--no-link` is used.
