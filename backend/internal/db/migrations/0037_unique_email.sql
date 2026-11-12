-- An address names at most one account, in any case; any number of accounts
-- may have none.
ALTER TABLE users
    ADD COLUMN email_key VARCHAR(255) COLLATE utf8mb4_bin AS (NULLIF(LOWER(email), '')) PERSISTENT,
    ADD UNIQUE KEY uq_users_email_key (email_key);
