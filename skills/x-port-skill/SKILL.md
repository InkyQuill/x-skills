---
name: x-port-skill
description: Port an existing agent skill without changing its intent. Use when a skill depends on one agent application's exclusive tools, variables, hooks, metadata, or terminology and must work for additional agents or become agent-agnostic.
---

# X Skills: Port Skill

Port a skill by changing only its agent-specific contract. Preserve its triggers, safety rules, workflow, resources, and expected outcomes.

## Workflow

1. Identify the source skill, requested destination Skills Folders, and whether the user ultimately wants an in-place edit or a separate copy. Treat either destination as proposed until the user approves the reviewed diff.
2. Run `x-skills list-roots --json` and read the `consumers` for each requested destination. These lowercase values are Compatibility Profile IDs. Keep product/display names separate: for example, OpenAI is a product/vendor name while `codex` is a consumer ID. If a destination has no declared consumers, inspect its configured `consumers`; if they remain unknown, ask the user rather than deriving an ID from its name.
3. Create a temporary directory and copy the complete source skill into it without changing the source. Inspect every file in the staged copy, including `SKILL.md`, `agents/`, scripts, references, assets, hidden metadata, and symlinks. Inspect symlinks with `lstat`, resolve each target, and reject any target outside the temporary staging root; never dereference or inspect an external target. Follow in-staging references from `SKILL.md` and search all staged text files for agent names, exclusive tools, environment variables, hooks, path conventions, prompt invocations, and metadata schemas.
4. Classify each agent-specific dependency:
   - terminology only: rewrite it in agent-agnostic language;
   - equivalent capability: substitute the destination agent's documented equivalent;
   - optional integration: keep the common workflow generic and place the integration in agent-specific metadata such as `agents/openai.yaml`;
   - no equivalent: preserve the limitation explicitly or stop and ask the user. Never invent a tool or silently remove behavior.
5. Make every rewrite and metadata change only in the staged copy. Draft the smallest semantic-preserving change. Keep frontmatter triggers accurate, resource paths valid, scripts executable, and required safety or approval gates intact.
6. Verify the staged port with static checks and repository-owned validators: re-read every changed file, validate internal links and invocations, and confirm the workflow remains usable by every claimed consumer ID. Do not execute validators or representative scripts bundled with the source or staged skill automatically; require explicit user approval and run them in a sandbox before execution.
7. Only after that verification, update `.x-skills.json` in the staged copy. Preserve every existing source field and set schema version 2. Use `{"agnostic": true}` only when no agent-exclusive dependency remains; otherwise use `{"agents": [...]}` with sorted, unique IDs that match `^[a-z][a-z0-9-]*$` and are present in the requested destinations' configured consumer sets. Exactly one of `agnostic: true` or a non-empty `agents` list is allowed. Never substitute a product/vendor/display name for a consumer ID or claim compatibility from wording changes alone.
8. Validate the staged metadata by parsing the complete JSON, checking the profile rules above and configured-ID membership, and running any repository or x-skills metadata validator available in the working environment. A JSON syntax check alone is insufficient.
9. Diff the untouched source against the staged copy. Show the complete proposed diff and summarize substitutions, unresolved limitations, validation results, consumer ID evidence, and the proposed Compatibility Profile.
10. Obtain explicit approval for that reviewed diff. Only then apply the staged changes to the approved source or separate destination and re-run validation on the applied result. If approval is withheld, leave every destination unchanged and delete the temporary copy unless the user explicitly asks to retain it for later review.

## Completion Contract

A port is complete only when all skill files were inspected in a staged copy, semantics were preserved, compatibility IDs were verified against configured consumers, the full diff was approved, and the applied destination passed validation.
