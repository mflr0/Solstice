-- Grants become allow and deny rules, per project or global (project_id NULL).
-- The most specific level with a rule decides: a user rule of the project, a
-- global user rule, the group rules of the project (built-in groups included:
-- everyone, lead, reporter, assignee), then the global group rules. Within a
-- group level, deny beats allow.
CREATE TABLE permission_rules (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NULL,
    permission VARCHAR(40) NOT NULL,
    effect     ENUM('allow', 'deny') NOT NULL,
    holder     VARCHAR(20) NOT NULL,
    group_id   BIGINT NULL,
    user_id    BIGINT NULL,
    KEY ix_pr_project (project_id, permission),
    KEY ix_pr_group (group_id),
    CONSTRAINT fk_pr_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_pr_group FOREIGN KEY (group_id) REFERENCES user_groups (id) ON DELETE CASCADE,
    CONSTRAINT fk_pr_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

INSERT INTO permission_rules (project_id, permission, effect, holder, group_id, user_id)
SELECT project_id, permission, 'allow', holder, group_id, user_id FROM project_permissions ORDER BY id;

DROP TABLE project_permissions;
