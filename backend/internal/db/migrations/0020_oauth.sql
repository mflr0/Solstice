-- OAuth authorization of MCP clients. A grant is what a user allowed one
-- client (its Client ID Metadata Document URL); client_key is that URL's
-- SHA-256, for the unique key. Scopes are names separated by spaces, as in
-- api_tokens.
CREATE TABLE oauth_grants (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id      BIGINT NOT NULL,
    client_id    VARCHAR(1024) COLLATE utf8mb4_bin NOT NULL,
    client_key   CHAR(64) NOT NULL,
    client_name  VARCHAR(255) NOT NULL,
    scopes       VARCHAR(255) NOT NULL,
    created_at   DATETIME(3) NOT NULL,
    last_used_at DATETIME(3) NULL,
    UNIQUE KEY uq_oauth_grants_client (user_id, client_key),
    CONSTRAINT fk_oauth_grants_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Authorization codes, kept once used until they expire so that a replay is
-- recognised. A code starts a family: the tokens it and their refreshes give.
CREATE TABLE oauth_codes (
    id             BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    grant_id       BIGINT NOT NULL,
    code_hash      CHAR(64) NOT NULL,
    redirect_uri   VARCHAR(1024) COLLATE utf8mb4_bin NOT NULL,
    code_challenge VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    scopes         VARCHAR(255) NOT NULL,
    expires_at     DATETIME(3) NOT NULL,
    used_at        DATETIME(3) NULL,
    UNIQUE KEY uq_oauth_codes_hash (code_hash),
    CONSTRAINT fk_oauth_codes_grant FOREIGN KEY (grant_id) REFERENCES oauth_grants (id) ON DELETE CASCADE
);

-- Access and refresh tokens (kind), hashed. A refresh token is kept once
-- rotated (used_at) until it expires, so that its reuse revokes its family.
CREATE TABLE oauth_tokens (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    grant_id   BIGINT NOT NULL,
    family     BIGINT NOT NULL,
    kind       VARCHAR(10) NOT NULL,
    token_hash CHAR(64) NOT NULL,
    scopes     VARCHAR(255) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    used_at    DATETIME(3) NULL,
    UNIQUE KEY uq_oauth_tokens_hash (token_hash),
    KEY ix_oauth_tokens_family (family),
    CONSTRAINT fk_oauth_tokens_grant FOREIGN KEY (grant_id) REFERENCES oauth_grants (id) ON DELETE CASCADE
);
