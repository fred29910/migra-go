CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) DEFAULT 'unknown',
    created_at TIMESTAMP DEFAULT now()
);
