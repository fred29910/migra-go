-- Example target schema (modified from source)
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) DEFAULT 'unknown',
    created_at TIMESTAMP DEFAULT now(),
    age INTEGER  -- New column added
);

CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_posts_user_id ON posts(user_id);
CREATE UNIQUE INDEX idx_users_username ON users(username);

-- Renamed enum type
CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest');  -- Added 'guest'

-- New table
CREATE TABLE comments (
    id SERIAL PRIMARY KEY,
    post_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT now()
);
