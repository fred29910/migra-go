# v3 — Modification and Drop Scenario

This version represents a schema evolution from v2 with destructive changes:

- **users**: dropped `age` column, added `phone VARCHAR(20)`, added table-level `UNIQUE (email)` constraint
- **posts**: unchanged from v2
- **comments**: **dropped entirely** (table removed)
- **indexes**: renamed `idx_users_username` to `idx_users_username_unique` (drop old, create new)
- **user_role enum**: removed `'guest'`, added `'moderator'`
- **categories**: new table (`id`, `name` with UNIQUE, `sort_order` with default 0)

Used for testing diff scenarios that involve column drops, table drops, enum modifications, index renames, and new table additions.
