-- Former user names of renamed accounts: mentions, filters and lookups
-- written under an old name keep finding the account. Lower case, like
-- users.username.
CREATE TABLE user_aliases (
    name    VARCHAR(255) COLLATE utf8mb4_bin NOT NULL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    KEY ix_user_aliases_user (user_id),
    CONSTRAINT fk_user_aliases_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
