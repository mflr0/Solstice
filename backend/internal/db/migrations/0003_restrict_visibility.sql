-- Deleting a role or a security level must never make restricted content
-- public. With ON DELETE SET NULL it would have: a comment restricted to a
-- deleted role, or an issue at a deleted level, would become visible to all.
-- Such deletions are now refused while something still refers to them
-- (delete or move that content first).

ALTER TABLE comments DROP FOREIGN KEY fk_comments_role;
ALTER TABLE comments ADD CONSTRAINT fk_comments_role
    FOREIGN KEY (visibility_role_id) REFERENCES project_roles (id);

ALTER TABLE worklogs DROP FOREIGN KEY fk_worklogs_role;
ALTER TABLE worklogs ADD CONSTRAINT fk_worklogs_role
    FOREIGN KEY (visibility_role_id) REFERENCES project_roles (id);

ALTER TABLE issues DROP FOREIGN KEY fk_issues_security;
ALTER TABLE issues ADD CONSTRAINT fk_issues_security
    FOREIGN KEY (security_level_id) REFERENCES security_levels (id);
