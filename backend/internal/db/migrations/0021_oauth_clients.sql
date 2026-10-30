-- OAuth clients registered through Dynamic Client Registration (RFC 7591),
-- beside those known by a metadata document. redirect_uris is a JSON array.
-- authorized_at is set at the first consent: a client never authorized is
-- deleted a day after it registered.
CREATE TABLE oauth_clients (
    id            BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    client_id     VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
    client_name   VARCHAR(255) NOT NULL,
    redirect_uris TEXT NOT NULL,
    created_at    DATETIME(3) NOT NULL,
    authorized_at DATETIME(3) NULL,
    UNIQUE KEY uq_oauth_clients_id (client_id),
    KEY ix_oauth_clients_unused (authorized_at, created_at)
);
