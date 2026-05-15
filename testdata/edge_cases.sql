-- Edge case SQL patterns
-- Tests parser robustness with unusual but valid PostgreSQL DDL patterns

-- Table with quoted identifiers (mixed-case names require quoting)
CREATE TABLE "MixedCase" (
    "Id" SERIAL PRIMARY KEY,
    "Full Name" VARCHAR(100)
);

-- Table without columns (unusual but valid in PostgreSQL)
CREATE TABLE empty_table ();

-- Table with ONLY clause (table inheritance)
CREATE TABLE child_table () INHERITS (users);

-- Table with partitioning clause
CREATE TABLE partitioned_table (
    id SERIAL PRIMARY KEY,
    created_at DATE NOT NULL
) PARTITION BY RANGE (created_at);

-- IF NOT EXISTS clause
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY
);
