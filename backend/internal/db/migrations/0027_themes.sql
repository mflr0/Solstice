-- Themes imported on this instance; the files are in <data>/themes/<id>.
-- id is the id the theme is served under, manifest_id the one in its package:
-- they differ when another theme already holds the latter. imported_by is a
-- user name, not a key: a Jira import replaces the users and keeps this table.
CREATE TABLE themes (
    id          VARCHAR(32) COLLATE utf8mb4_bin NOT NULL PRIMARY KEY,
    manifest_id VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
    manifest    MEDIUMTEXT NOT NULL,
    version     VARCHAR(255) NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    imported_by VARCHAR(255) NULL,
    imported_at DATETIME(3) NOT NULL
);
