-- What each personal access token may do: scope names separated by spaces
-- (auth.Scopes). Tokens made before scopes existed had every right: they are
-- deleted, nothing having been released yet.
DELETE FROM api_tokens;
ALTER TABLE api_tokens ADD COLUMN scopes VARCHAR(255) NOT NULL AFTER token_hash;
