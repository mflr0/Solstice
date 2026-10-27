-- The OpenID Connect identity an account is linked to: the provider's
-- issuer and its stable subject (sub claim) for the person. NULL until the
-- account first signs in with OpenID Connect. users.directory may now also
-- be 'oidc', for an account created by an OpenID Connect sign-in.
ALTER TABLE users
    ADD COLUMN oidc_issuer VARCHAR(255) COLLATE utf8mb4_bin NULL,
    ADD COLUMN oidc_subject VARCHAR(255) COLLATE utf8mb4_bin NULL,
    ADD UNIQUE KEY uq_users_oidc (oidc_issuer, oidc_subject);
