-- Single-use links sent by e-mail: password reset ("forgot your password",
-- or sent by an administrator) and invitation of a new account. Only a
-- SHA-256 hash of each token is stored; the token itself exists in the e-mail
-- only. A token is used once (used_at), expires (expires_at) and is replaced
-- by the next one issued for the same account.
CREATE TABLE account_tokens (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT NOT NULL,
    token_hash CHAR(64) NOT NULL,
    purpose    VARCHAR(10) NOT NULL,  -- reset, invite
    created_at DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    used_at    DATETIME(3) NULL,
    UNIQUE KEY uq_account_tokens_hash (token_hash),
    KEY ix_account_tokens_user (user_id, used_at),
    KEY ix_account_tokens_expires (expires_at),
    CONSTRAINT fk_account_tokens_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
