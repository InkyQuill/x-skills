# Make Install a top-level TUI page

Remote discovery and install flows live in a top-level `I:Install` page instead of a Repo modal or command palette flow. Search, preview, audit status, archive state, conflict resolution, and optional immediate linking need enough persistent state that hiding them inside Repo would make navigation and keyboard scope unclear.

The Install page follows the same global page schema as Active, Repo, and Doctor. Page actions operate on Install selections with cursor fallback, while Repo remains focused on already archived skills and their update state.
