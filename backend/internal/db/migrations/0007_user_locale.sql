-- The interface language each user chose (fr, en, de, it, hi). NULL follows
-- the browser's language.

ALTER TABLE users ADD COLUMN locale VARCHAR(10) NULL;
