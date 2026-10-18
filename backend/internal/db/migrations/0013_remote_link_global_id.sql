-- The globalId of a remote link, by which the application that made it finds,
-- updates and removes it (Jira's REST API). Unique per issue and compared
-- exactly; links without one keep NULL, which a unique index allows many times.
ALTER TABLE remote_links
    ADD COLUMN global_id VARCHAR(255) COLLATE utf8mb4_bin NULL,
    ADD UNIQUE KEY uq_remote_links_global_id (issue_id, global_id);
