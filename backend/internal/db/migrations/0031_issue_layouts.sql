-- The fields of an issue type's create and view forms, in order: the type's
-- template (project_id NULL) or a project's override of it. A template
-- without rows is not customised. A unique key holds NULLs apart, so
-- project_key and field_key, never NULL, keep one row per field per template.
CREATE TABLE issue_layout_fields (
    id              BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id      BIGINT NULL,
    issue_type_id   VARCHAR(60) NOT NULL,
    form            ENUM('create', 'view') NOT NULL,
    system_field    VARCHAR(60) COLLATE utf8mb4_bin NULL,
    custom_field_id BIGINT NULL,
    zone            ENUM('main', 'side') NULL,
    position        INT NOT NULL,
    hidden          BOOLEAN NOT NULL DEFAULT FALSE,
    required        BOOLEAN NOT NULL DEFAULT FALSE,
    project_key     BIGINT AS (COALESCE(project_id, 0)) PERSISTENT,
    field_key       VARCHAR(80) COLLATE utf8mb4_bin AS (COALESCE(system_field, CONCAT('customfield_', custom_field_id))) PERSISTENT,
    UNIQUE KEY ux_ilf_field (issue_type_id, project_key, form, field_key),
    KEY ix_ilf_project (project_id),
    KEY ix_ilf_custom (custom_field_id),
    CONSTRAINT ck_ilf_field CHECK ((system_field IS NULL) <> (custom_field_id IS NULL)),
    CONSTRAINT fk_ilf_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_ilf_type FOREIGN KEY (issue_type_id) REFERENCES issue_types (id) ON DELETE CASCADE,
    CONSTRAINT fk_ilf_custom FOREIGN KEY (custom_field_id) REFERENCES custom_fields (id) ON DELETE CASCADE
);
