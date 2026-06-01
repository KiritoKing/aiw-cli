Generate a Git commit message for the staged diff.

Rules:
- Output only the commit message. Do not add Markdown fences or explanations.
- Prefer Conventional Commits when the change clearly fits one type.
- Keep the first line concise and specific.
- Use a body only when it clarifies non-obvious context or hook failures.
- Do not mention generated tooling unless it is part of the change.
