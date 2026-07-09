# Remote skill identity uses source metadata with name fallback

Remote install and update flows identify an archived skill by stored GitHub source metadata when available, and fall back to skill name only for legacy or manually archived skills without metadata. This avoids treating different upstream skills with the same name as identical while still letting existing archives show useful `archived` or `name conflict` states in the Install view.
