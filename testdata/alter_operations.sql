-- ALTER TABLE operations test cases
-- Comprehensive coverage of all ALTER TABLE variants for parser testing

CREATE TABLE items (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price NUMERIC(10,2) DEFAULT 0.00,
    quantity INTEGER DEFAULT 0,
    category VARCHAR(50),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP
);

-- Add new column
ALTER TABLE items ADD COLUMN sku VARCHAR(50);

-- Alter column type (varchar -> text)
ALTER TABLE items ALTER COLUMN name TYPE TEXT;

-- Alter column type with USING clause
ALTER TABLE items ALTER COLUMN price TYPE DOUBLE PRECISION USING price::double precision;

-- Set NOT NULL on previously nullable column
ALTER TABLE items ALTER COLUMN description SET NOT NULL;

-- Drop NOT NULL
ALTER TABLE items ALTER COLUMN category DROP NOT NULL;

-- Set default value
ALTER TABLE items ALTER COLUMN updated_at SET DEFAULT now();

-- Drop default value
ALTER TABLE items ALTER COLUMN price DROP DEFAULT;

-- Add column with NOT NULL and default
ALTER TABLE items ADD COLUMN tags TEXT[] DEFAULT '{}';

-- Multiple ALTER TABLE operations in sequence
ALTER TABLE items ADD COLUMN sort_order INTEGER;
ALTER TABLE items ALTER COLUMN sort_order SET DEFAULT 0;
ALTER TABLE items ALTER COLUMN sort_order SET NOT NULL;

-- Rename column (PostgreSQL syntax)
ALTER TABLE items RENAME COLUMN sku TO item_code;

-- Set storage parameter
ALTER TABLE items SET (fillfactor = 70);

-- Add column with complex default
ALTER TABLE items ADD COLUMN uuid_col UUID DEFAULT gen_random_uuid();
