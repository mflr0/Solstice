-- Accounts of an external directory. users.directory: '' for a local account
-- (its password is password_hash), 'ldap' for one that signs in through LDAP
-- (no password here). user_groups.from_directory: the group's members among
-- directory accounts are set by the directory at each sign-in.
ALTER TABLE users ADD COLUMN directory VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE user_groups ADD COLUMN from_directory BOOLEAN NOT NULL DEFAULT FALSE;
