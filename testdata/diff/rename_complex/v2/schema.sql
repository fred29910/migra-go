CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    login_name VARCHAR(50) NOT NULL,
    email VARCHAR(100) DEFAULT 'unknown' UNIQUE,
    created_at TIMESTAMP DEFAULT now(),
    phone VARCHAR(20)
);

CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_posts_user_id ON posts(user_id);
CREATE UNIQUE INDEX idx_users_login_name ON users(login_name);

CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest');