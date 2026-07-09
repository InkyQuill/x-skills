# Use the legacy skills.sh search endpoint for unauthenticated discovery

Remote discovery uses the unauthenticated `https://skills.sh/api/search` endpoint with a minimum two-character query and a bounded result limit, defaulting to 50. The newer `/api/v1` API is intentionally not the first implementation target because it requires authentication, while the current Vercel CLI also uses the legacy endpoint without server pagination.
