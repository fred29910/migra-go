-- UTF-8 encoded SQL with special characters
CREATE TABLE utf8_test (
    id SERIAL PRIMARY KEY,
    label VARCHAR(100),
    description TEXT
);

-- Comments with Unicode: 日本語, 中文, Español, Français
-- Column with accented name
ALTER TABLE utf8_test ADD COLUMN "café" VARCHAR(50);
