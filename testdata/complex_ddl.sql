-- Complex DDL test cases
-- Tests parser's handling of composite keys, multiple constraint types,
-- advanced data types, and custom enum types

-- Table with composite primary key
CREATE TABLE order_items (
    order_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 1,
    price NUMERIC(12,2) NOT NULL,
    PRIMARY KEY (order_id, item_id)
);

-- Table with multiple constraint types
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    sku VARCHAR(50) NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    price NUMERIC(10,2) NOT NULL CHECK (price > 0),
    stock INTEGER DEFAULT 0 CHECK (stock >= 0),
    category_id INTEGER NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    updated_at TIMESTAMPTZ DEFAULT now(),
    CONSTRAINT products_sku_unique UNIQUE (sku),
    CONSTRAINT products_category_fkey FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Table with various data types
CREATE TABLE analytics (
    id BIGSERIAL PRIMARY KEY,
    event_name VARCHAR(100) NOT NULL,
    user_id INTEGER,
    session_id UUID,
    ip_address INET,
    metadata JSONB DEFAULT '{}',
    duration INTERVAL,
    score REAL,
    rating SMALLINT CHECK (rating >= 1 AND rating <= 5),
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Enum types for reference
CREATE TYPE user_status AS ENUM ('active', 'inactive', 'suspended');
CREATE TYPE content_type AS ENUM ('article', 'video', 'image', 'podcast');

-- Table using custom enum types
CREATE TABLE content_items (
    id SERIAL PRIMARY KEY,
    title VARCHAR(300) NOT NULL,
    content_type content_type NOT NULL,
    status user_status DEFAULT 'active',
    author_id INTEGER NOT NULL,
    published_at TIMESTAMPTZ,
    CONSTRAINT content_author_fkey FOREIGN KEY (author_id) REFERENCES users(id)
);
