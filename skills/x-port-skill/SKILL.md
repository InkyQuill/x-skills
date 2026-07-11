---
name: x-port-skill
description: Port an existing agent skill without changing its intent. Use when a skill depends on one agent application's exclusive tools, variables, hooks, metadata, or terminology and must work for additional agents or become agent-agnostic.
---

# X Skills: Port Skill

Port a skill by changing only its agent-specific contract. Preserve its triggers, safety rules, workflow, resources, and expected outcomes.

## Workflow

1. Identify the source skill, requested destination agents, and whether the user wants an in-place edit or a separate copy. Treat an unspecified overwrite as a proposal, never as permission.
2. Inspect every file in the skill directory, including `SKILL.md`, `agents/`, scripts, references, assets, hidden metadata, and symlinks. Follow references from `SKILL.md` and search all text files for agent names, exclusive tools, environment variables, hooks, path conventions, prompt invocations, and metadata schemas.
3. Classify each agent-specific dependency:
   - terminology only: rewrite it in agent-agnostic language;
   - equivalent capability: substitute the destination agent's documented equivalent;
   - optional integration: keep the common workflow generic and place the integration in agent-specific metadata such as `agents/openai.yaml`;
   - no equivalent: preserve the limitation explicitly or stop and ask the user. Never invent a tool or silently remove behavior.
4. Draft the smallest semantic-preserving change. Keep frontmatter triggers accurate, resource paths valid, scripts executable, and required safety or approval gates intact.
5. Verify the complete port: re-read every changed file, validate internal links and invocations, run bundled validators or representative scripts, and confirm the workflow remains usable by every claimed agent.
6. Only after that verification, add or update the x-skills Compatibility Profile in `.x-skills.json`: use `{"agnostic": true}` only when no agent-exclusive dependency remains; otherwise list only agents actually verified. Preserve all existing source metadata and never claim compatibility from wording changes alone.
7. Show the proposed diff and summarize substitutions, unresolved limitations, validation results, and the proposed Compatibility Profile.
8. Obtain explicit approval before overwriting the source skill. If approval is absent, leave the source unchanged and provide the diff or create a separate copy only when that destination was already authorized.

## Completion Contract

A port is complete only when all skill files were inspected, semantics were preserved, compatibility claims were verified, the full diff was shown, and any overwrite was explicitly approved.
