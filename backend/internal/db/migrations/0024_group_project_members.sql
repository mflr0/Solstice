-- Project roles are replaced by groups with project members. Being in a group
-- in a project means a global member (group_members), or a project member of
-- that project: a user, or a member of a member group in that project.
-- Roles are dropped, not converted, together with whatever pointed at them.
CREATE TABLE group_project_members (
    id              BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    group_id        BIGINT NOT NULL,
    project_id      BIGINT NOT NULL,
    user_id         BIGINT NULL,
    member_group_id BIGINT NULL,
    UNIQUE KEY uq_gpm_user (group_id, project_id, user_id),
    UNIQUE KEY uq_gpm_group (group_id, project_id, member_group_id),
    KEY ix_gpm_project (project_id, group_id),
    KEY ix_gpm_user (user_id),
    CONSTRAINT fk_gpm_group FOREIGN KEY (group_id) REFERENCES user_groups (id) ON DELETE CASCADE,
    CONSTRAINT fk_gpm_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_gpm_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_gpm_member_group FOREIGN KEY (member_group_id) REFERENCES user_groups (id) ON DELETE CASCADE
);

-- Deleted rather than opened up: an item restricted to a role must never
-- become visible to more people.
DELETE FROM comments WHERE visibility_role_id IS NOT NULL;
DELETE FROM worklogs WHERE visibility_role_id IS NOT NULL;
DELETE FROM project_permissions WHERE role_id IS NOT NULL OR holder = 'role';
DELETE FROM security_level_members WHERE role_id IS NOT NULL OR holder = 'role';
DELETE FROM filter_shares WHERE role_id IS NOT NULL;
DELETE FROM project_notification_recipients WHERE role_id IS NOT NULL;

ALTER TABLE comments DROP FOREIGN KEY fk_comments_role;
ALTER TABLE comments DROP COLUMN visibility_role_id;
ALTER TABLE worklogs DROP FOREIGN KEY fk_worklogs_role;
ALTER TABLE worklogs DROP COLUMN visibility_role_id;
ALTER TABLE project_permissions DROP FOREIGN KEY fk_pp_role;
ALTER TABLE project_permissions DROP COLUMN role_id;
ALTER TABLE security_level_members DROP FOREIGN KEY fk_slm_role;
ALTER TABLE security_level_members DROP COLUMN role_id;
ALTER TABLE filter_shares DROP FOREIGN KEY fk_fs_role;
ALTER TABLE filter_shares DROP COLUMN role_id;
ALTER TABLE project_notification_recipients DROP FOREIGN KEY fk_pnr_role;
ALTER TABLE project_notification_recipients DROP COLUMN role_id;

DROP TABLE project_role_members;
DROP TABLE project_roles;
