-- Personal access tokens and the audit log.

-- Scripts and tools sign in with a personal access token (HTTP
-- "Authorization: Bearer", as with Jira Data Center). Only a SHA-256 hash
-- of each token is stored; the token itself is shown once, at creation.
CREATE TABLE api_tokens (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id      BIGINT NOT NULL,
    name         VARCHAR(255) NOT NULL,
    token_hash   CHAR(64) NOT NULL,
    created_at   DATETIME(3) NOT NULL,
    expires_at   DATETIME(3) NULL,  -- NULL: never
    last_used_at DATETIME(3) NULL,
    UNIQUE KEY uq_api_tokens_hash (token_hash),
    KEY ix_api_tokens_user (user_id),
    CONSTRAINT fk_api_tokens_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Who changed what in accounts, access rights and configuration, and
-- sign-ins. Entries outlive their author's account (user_id is cleared, the
-- user name stays). The retention period is the setting
-- "audit.retention_days" (table settings; 365 days by default).
CREATE TABLE audit_log (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    created_at   DATETIME(3) NOT NULL,
    user_id      BIGINT NULL,
    username     VARCHAR(255) NULL,  -- the author's, or the name typed for a failed sign-in
    ip           VARCHAR(64) NULL,
    action       VARCHAR(100) NOT NULL,  -- e.g. user.create, token.revoke
    target_type  VARCHAR(50) NULL,
    target_id    VARCHAR(100) NULL,
    target_label VARCHAR(255) NULL,
    details      MEDIUMTEXT NULL,  -- JSON, never secrets
    KEY ix_audit_created (created_at),
    KEY ix_audit_user (user_id, created_at),
    KEY ix_audit_action (action, created_at),
    CONSTRAINT fk_audit_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
);
