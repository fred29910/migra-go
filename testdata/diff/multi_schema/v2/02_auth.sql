CREATE SCHEMA auth;

CREATE TABLE auth.roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT DEFAULT ''
);

CREATE TABLE auth.permissions (
    id SERIAL PRIMARY KEY,
    role_id INTEGER NOT NULL REFERENCES auth.roles(id),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL
);
