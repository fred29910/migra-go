-- DROP operations test scenarios
-- These represent a schema after objects have been dropped.
-- The diff engine detects DROPs when these exist in source but not here.

-- Table that still exists
CREATE TABLE active_items (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL
);

-- New index on remaining table
CREATE INDEX idx_active_items_name ON active_items(name);

CREATE TYPE item_status AS ENUM ('active', 'archived');
