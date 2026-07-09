# Use git CLI for Git source transport

GitHub and generic Git installs, previews, and update checks use the local `git` executable: `git clone` for temporary checkouts, `git rev-parse HEAD` for installed commit metadata, and `git ls-remote` for update checks. This keeps the Go implementation provider-neutral, records exact source commits, and avoids coupling the first remote-install slice to authenticated skills.sh APIs or GitHub archive-download behavior.
