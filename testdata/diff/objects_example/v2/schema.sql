CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) DEFAULT 'unknown'
);

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE SEQUENCE custom_order_seq
    INCREMENT BY 1
    START WITH 1000
    MINVALUE 1
    MAXVALUE 999999
    CACHE 1;
CREATE VIEW active_users AS
    SELECT id, username, email
    FROM users
    WHERE email IS NOT NULL;
