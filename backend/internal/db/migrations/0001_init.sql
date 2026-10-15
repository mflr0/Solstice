-- Initial schema.
-- Every date is stored in UTC. Identifiers coming from Jira are kept on import.
-- No "schemes": permissions, issue types and security levels belong to each project.

CREATE TABLE users (
    id            BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    username      VARCHAR(255) COLLATE utf8mb4_bin NOT NULL,  -- login, lower case
    user_key      VARCHAR(255) COLLATE utf8mb4_bin NULL,      -- original Jira key (NULL for accounts created in Solstice)
    display_name  VARCHAR(255) NOT NULL,
    email         VARCHAR(255) NOT NULL DEFAULT '',
    password_hash VARCHAR(255) NULL,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    is_admin      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uq_users_username (username),
    UNIQUE KEY uq_users_user_key (user_key)
);

CREATE TABLE user_groups (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(255) COLLATE utf8mb4_bin NOT NULL,  -- lower case
    description VARCHAR(255) NULL,
    UNIQUE KEY uq_user_groups_name (name)
);

-- Effective membership: Jira's nested groups are flattened on import.
CREATE TABLE group_members (
    group_id BIGINT NOT NULL,
    user_id  BIGINT NOT NULL,
    PRIMARY KEY (group_id, user_id),
    KEY ix_group_members_user (user_id),
    CONSTRAINT fk_gm_group FOREIGN KEY (group_id) REFERENCES user_groups (id) ON DELETE CASCADE,
    CONSTRAINT fk_gm_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE sessions (
    token_hash CHAR(64) NOT NULL PRIMARY KEY,  -- SHA-256 of the cookie token
    user_id    BIGINT NOT NULL,
    created_at DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    KEY ix_sessions_user (user_id),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Global catalogues ----------------------------------------------------------

CREATE TABLE issue_types (
    id          VARCHAR(60) NOT NULL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT NULL,
    subtask     BOOLEAN NOT NULL DEFAULT FALSE,
    position    INT NOT NULL DEFAULT 0
);

CREATE TABLE statuses (
    id          VARCHAR(60) NOT NULL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT NULL,
    category    ENUM('todo', 'doing', 'done') NOT NULL,
    position    INT NOT NULL DEFAULT 0
);

CREATE TABLE priorities (
    id          VARCHAR(60) NOT NULL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT NULL,
    color       VARCHAR(20) NULL,
    position    INT NOT NULL DEFAULT 0
);

CREATE TABLE resolutions (
    id          VARCHAR(60) NOT NULL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT NULL,
    position    INT NOT NULL DEFAULT 0
);

CREATE TABLE link_types (
    id       BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name     VARCHAR(255) NOT NULL,
    inward   VARCHAR(255) NOT NULL,
    outward  VARCHAR(255) NOT NULL,
    position INT NOT NULL DEFAULT 0
);

CREATE TABLE project_roles (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT NULL
);

CREATE TABLE custom_fields (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    type_key     VARCHAR(255) NOT NULL,
    searcher_key VARCHAR(255) NULL,
    name         VARCHAR(255) NOT NULL,
    description  TEXT NULL
);

CREATE TABLE custom_field_options (
    id        BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    field_id  BIGINT NOT NULL,
    parent_id BIGINT NULL,
    position  INT NOT NULL DEFAULT 0,
    value     VARCHAR(255) NOT NULL,
    disabled  BOOLEAN NOT NULL DEFAULT FALSE,
    KEY ix_cfo_field (field_id),
    CONSTRAINT fk_cfo_field FOREIGN KEY (field_id) REFERENCES custom_fields (id) ON DELETE CASCADE,
    CONSTRAINT fk_cfo_parent FOREIGN KEY (parent_id) REFERENCES custom_field_options (id) ON DELETE CASCADE
);

-- Projects -------------------------------------------------------------------

CREATE TABLE project_categories (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT NULL
);

CREATE TABLE projects (
    id               BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    pkey             VARCHAR(255) COLLATE utf8mb4_bin NOT NULL,
    name             VARCHAR(255) NOT NULL,
    description      TEXT NULL,
    lead_id          BIGINT NULL,
    category_id      BIGINT NULL,
    url              VARCHAR(255) NULL,
    kind             VARCHAR(60) NOT NULL DEFAULT 'software',
    default_assignee ENUM('unassigned', 'lead') NOT NULL DEFAULT 'unassigned',
    issue_counter    BIGINT NOT NULL DEFAULT 0,
    UNIQUE KEY uq_projects_pkey (pkey),
    CONSTRAINT fk_projects_lead FOREIGN KEY (lead_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT fk_projects_category FOREIGN KEY (category_id) REFERENCES project_categories (id) ON DELETE SET NULL
);

-- Former keys of a renamed project (WEB used to be SITE): /browse/SITE-12 redirects.
CREATE TABLE project_keys (
    pkey       VARCHAR(255) COLLATE utf8mb4_bin NOT NULL PRIMARY KEY,
    project_id BIGINT NOT NULL,
    CONSTRAINT fk_project_keys_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);

CREATE TABLE project_issue_types (
    project_id    BIGINT NOT NULL,
    issue_type_id VARCHAR(60) NOT NULL,
    position      INT NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, issue_type_id),
    CONSTRAINT fk_pit_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_pit_type FOREIGN KEY (issue_type_id) REFERENCES issue_types (id) ON DELETE CASCADE
);

CREATE TABLE project_role_members (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    role_id    BIGINT NOT NULL,
    user_id    BIGINT NULL,
    group_id   BIGINT NULL,
    KEY ix_prm_project (project_id, role_id),
    CONSTRAINT fk_prm_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_prm_role FOREIGN KEY (role_id) REFERENCES project_roles (id) ON DELETE CASCADE,
    CONSTRAINT fk_prm_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_prm_group FOREIGN KEY (group_id) REFERENCES user_groups (id) ON DELETE CASCADE
);

-- Who may do what in a project.
-- holder: everyone (any signed-in user), group, user, role, lead, reporter, assignee.
CREATE TABLE project_permissions (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    permission VARCHAR(40) NOT NULL,
    holder     VARCHAR(20) NOT NULL,
    user_id    BIGINT NULL,
    group_id   BIGINT NULL,
    role_id    BIGINT NULL,
    KEY ix_pp_project (project_id, permission),
    CONSTRAINT fk_pp_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_pp_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_pp_group FOREIGN KEY (group_id) REFERENCES user_groups (id) ON DELETE CASCADE,
    CONSTRAINT fk_pp_role FOREIGN KEY (role_id) REFERENCES project_roles (id) ON DELETE CASCADE
);

-- Security levels of a project: a secured issue is only visible to the level's members.
CREATE TABLE security_levels (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id  BIGINT NOT NULL,
    name        VARCHAR(255) NOT NULL,
    description TEXT NULL,
    CONSTRAINT fk_sl_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);

CREATE TABLE security_level_members (
    id       BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    level_id BIGINT NOT NULL,
    holder   VARCHAR(20) NOT NULL,
    user_id  BIGINT NULL,
    group_id BIGINT NULL,
    role_id  BIGINT NULL,
    KEY ix_slm_level (level_id),
    CONSTRAINT fk_slm_level FOREIGN KEY (level_id) REFERENCES security_levels (id) ON DELETE CASCADE,
    CONSTRAINT fk_slm_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_slm_group FOREIGN KEY (group_id) REFERENCES user_groups (id) ON DELETE CASCADE,
    CONSTRAINT fk_slm_role FOREIGN KEY (role_id) REFERENCES project_roles (id) ON DELETE CASCADE
);

CREATE TABLE components (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id  BIGINT NOT NULL,
    name        VARCHAR(255) NOT NULL,
    description TEXT NULL,
    lead_id     BIGINT NULL,
    archived    BOOLEAN NOT NULL DEFAULT FALSE,
    KEY ix_components_project (project_id),
    CONSTRAINT fk_components_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_components_lead FOREIGN KEY (lead_id) REFERENCES users (id) ON DELETE SET NULL
);

CREATE TABLE versions (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id   BIGINT NOT NULL,
    name         VARCHAR(255) NOT NULL,
    description  TEXT NULL,
    position     INT NOT NULL DEFAULT 0,
    released     BOOLEAN NOT NULL DEFAULT FALSE,
    archived     BOOLEAN NOT NULL DEFAULT FALSE,
    start_date   DATE NULL,
    release_date DATE NULL,
    KEY ix_versions_project (project_id),
    CONSTRAINT fk_versions_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);

-- Issues ---------------------------------------------------------------------

CREATE TABLE issues (
    id                 BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id         BIGINT NOT NULL,
    num                BIGINT NOT NULL,
    issue_type_id      VARCHAR(60) NOT NULL,
    parent_id          BIGINT NULL,
    summary            VARCHAR(255) NOT NULL,
    description        LONGTEXT NULL,
    environment        LONGTEXT NULL,
    status_id          VARCHAR(60) NOT NULL,
    priority_id        VARCHAR(60) NULL,
    resolution_id      VARCHAR(60) NULL,
    reporter_id        BIGINT NULL,
    assignee_id        BIGINT NULL,
    creator_id         BIGINT NULL,
    security_level_id  BIGINT NULL,
    created_at         DATETIME(3) NOT NULL,
    updated_at         DATETIME(3) NOT NULL,
    resolved_at        DATETIME(3) NULL,
    due_date           DATE NULL,
    original_estimate  BIGINT NULL,  -- seconds
    remaining_estimate BIGINT NULL,
    time_spent         BIGINT NULL,
    archived           BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE KEY uq_issues_num (project_id, num),
    KEY ix_issues_assignee (assignee_id),
    KEY ix_issues_reporter (reporter_id),
    KEY ix_issues_status (status_id),
    KEY ix_issues_updated (updated_at),
    KEY ix_issues_parent (parent_id),
    FULLTEXT KEY ft_issues (summary, description),
    CONSTRAINT fk_issues_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_issues_type FOREIGN KEY (issue_type_id) REFERENCES issue_types (id),
    CONSTRAINT fk_issues_parent FOREIGN KEY (parent_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_issues_status FOREIGN KEY (status_id) REFERENCES statuses (id),
    CONSTRAINT fk_issues_priority FOREIGN KEY (priority_id) REFERENCES priorities (id),
    CONSTRAINT fk_issues_resolution FOREIGN KEY (resolution_id) REFERENCES resolutions (id),
    CONSTRAINT fk_issues_reporter FOREIGN KEY (reporter_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT fk_issues_assignee FOREIGN KEY (assignee_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT fk_issues_creator FOREIGN KEY (creator_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT fk_issues_security FOREIGN KEY (security_level_id) REFERENCES security_levels (id) ON DELETE SET NULL
);

-- An issue moved to another project: its old key redirects.
CREATE TABLE moved_issue_keys (
    old_key  VARCHAR(255) COLLATE utf8mb4_bin NOT NULL PRIMARY KEY,
    issue_id BIGINT NOT NULL,
    CONSTRAINT fk_mik_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE
);

CREATE TABLE issue_components (
    issue_id     BIGINT NOT NULL,
    component_id BIGINT NOT NULL,
    PRIMARY KEY (issue_id, component_id),
    KEY ix_ic_component (component_id),
    CONSTRAINT fk_ic_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_ic_component FOREIGN KEY (component_id) REFERENCES components (id) ON DELETE CASCADE
);

CREATE TABLE issue_versions (
    issue_id   BIGINT NOT NULL,
    version_id BIGINT NOT NULL,
    kind       ENUM('affects', 'fix') NOT NULL,
    PRIMARY KEY (issue_id, version_id, kind),
    KEY ix_iv_version (version_id),
    CONSTRAINT fk_iv_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_iv_version FOREIGN KEY (version_id) REFERENCES versions (id) ON DELETE CASCADE
);

CREATE TABLE labels (
    id       BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    issue_id BIGINT NOT NULL,
    field_id BIGINT NULL,  -- NULL: the issue's labels; otherwise a labels-type custom field
    label    VARCHAR(255) COLLATE utf8mb4_bin NOT NULL,
    KEY ix_labels_issue (issue_id),
    KEY ix_labels_label (label),
    CONSTRAINT fk_labels_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_labels_field FOREIGN KEY (field_id) REFERENCES custom_fields (id) ON DELETE CASCADE
);

CREATE TABLE issue_links (
    id        BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    type_id   BIGINT NOT NULL,
    source_id BIGINT NOT NULL,
    target_id BIGINT NOT NULL,
    KEY ix_links_source (source_id),
    KEY ix_links_target (target_id),
    CONSTRAINT fk_links_type FOREIGN KEY (type_id) REFERENCES link_types (id) ON DELETE CASCADE,
    CONSTRAINT fk_links_source FOREIGN KEY (source_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_links_target FOREIGN KEY (target_id) REFERENCES issues (id) ON DELETE CASCADE
);

CREATE TABLE remote_links (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    issue_id     BIGINT NOT NULL,
    url          TEXT NOT NULL,
    title        VARCHAR(255) NOT NULL,
    relationship VARCHAR(255) NULL,
    KEY ix_remote_links_issue (issue_id),
    CONSTRAINT fk_remote_links_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE
);

-- visibility_group / visibility_role_id: comment restricted to a group or a project role.
CREATE TABLE comments (
    id                 BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    issue_id           BIGINT NOT NULL,
    author_id          BIGINT NULL,
    body               LONGTEXT NOT NULL,
    created_at         DATETIME(3) NOT NULL,
    updated_at         DATETIME(3) NOT NULL,
    updated_by_id      BIGINT NULL,
    visibility_group   VARCHAR(255) COLLATE utf8mb4_bin NULL,
    visibility_role_id BIGINT NULL,
    KEY ix_comments_issue (issue_id, created_at),
    CONSTRAINT fk_comments_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_comments_author FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT fk_comments_updated_by FOREIGN KEY (updated_by_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT fk_comments_role FOREIGN KEY (visibility_role_id) REFERENCES project_roles (id) ON DELETE SET NULL
);

CREATE TABLE worklogs (
    id                 BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    issue_id           BIGINT NOT NULL,
    author_id          BIGINT NULL,
    body               LONGTEXT NULL,
    started_at         DATETIME(3) NOT NULL,
    seconds            BIGINT NOT NULL,
    created_at         DATETIME(3) NOT NULL,
    updated_at         DATETIME(3) NOT NULL,
    updated_by_id      BIGINT NULL,
    visibility_group   VARCHAR(255) COLLATE utf8mb4_bin NULL,
    visibility_role_id BIGINT NULL,
    KEY ix_worklogs_issue (issue_id),
    CONSTRAINT fk_worklogs_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_worklogs_author FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT fk_worklogs_updated_by FOREIGN KEY (updated_by_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT fk_worklogs_role FOREIGN KEY (visibility_role_id) REFERENCES project_roles (id) ON DELETE SET NULL
);

-- The file lives in <data>/attachments/<id>. file_missing: not found during import.
CREATE TABLE attachments (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    issue_id     BIGINT NOT NULL,
    author_id    BIGINT NULL,
    filename     VARCHAR(255) NOT NULL,
    mime_type    VARCHAR(255) NOT NULL,
    size         BIGINT NOT NULL,
    created_at   DATETIME(3) NOT NULL,
    file_missing BOOLEAN NOT NULL DEFAULT FALSE,
    KEY ix_attachments_issue (issue_id),
    CONSTRAINT fk_attachments_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_attachments_author FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE SET NULL
);

CREATE TABLE watchers (
    issue_id   BIGINT NOT NULL,
    user_id    BIGINT NOT NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (issue_id, user_id),
    KEY ix_watchers_user (user_id),
    CONSTRAINT fk_watchers_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_watchers_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE votes (
    issue_id   BIGINT NOT NULL,
    user_id    BIGINT NOT NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (issue_id, user_id),
    KEY ix_votes_user (user_id),
    CONSTRAINT fk_votes_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_votes_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Custom field values, stored the way Jira does (one row per value).
-- User fields: string_value holds the Solstice user id.
-- Date fields: date_value at midnight UTC of that day.
CREATE TABLE custom_field_values (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    issue_id     BIGINT NOT NULL,
    field_id     BIGINT NOT NULL,
    parent_key   VARCHAR(255) NULL,
    string_value VARCHAR(255) NULL,
    number_value DOUBLE NULL,
    text_value   LONGTEXT NULL,
    date_value   DATETIME(3) NULL,
    KEY ix_cfv_issue (issue_id),
    KEY ix_cfv_field (field_id, string_value),
    CONSTRAINT fk_cfv_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_cfv_field FOREIGN KEY (field_id) REFERENCES custom_fields (id) ON DELETE CASCADE
);

CREATE TABLE change_groups (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    issue_id   BIGINT NOT NULL,
    author_id  BIGINT NULL,
    created_at DATETIME(3) NOT NULL,
    KEY ix_cg_issue (issue_id, created_at),
    CONSTRAINT fk_cg_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE,
    CONSTRAINT fk_cg_author FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE SET NULL
);

CREATE TABLE change_items (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    group_id   BIGINT NOT NULL,
    field_type VARCHAR(255) NOT NULL,  -- jira | custom | solstice
    field      VARCHAR(255) NOT NULL,
    old_value  LONGTEXT NULL,
    old_string LONGTEXT NULL,
    new_value  LONGTEXT NULL,
    new_string LONGTEXT NULL,
    KEY ix_ci_group (group_id),
    CONSTRAINT fk_ci_group FOREIGN KEY (group_id) REFERENCES change_groups (id) ON DELETE CASCADE
);
