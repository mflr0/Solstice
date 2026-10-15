-- Administration from the interface: projects created, archived and
-- configured in Solstice, custom field contexts, last sign-ins.

-- An archived project is hidden from lists and pickers, and read-only.
ALTER TABLE projects ADD COLUMN archived BOOLEAN NOT NULL DEFAULT FALSE;

-- Who a new issue with this component goes to when nobody is chosen:
-- project_default, component_lead, project_lead, unassigned.
ALTER TABLE components ADD COLUMN assignee_type VARCHAR(20) NOT NULL DEFAULT 'project_default';

-- The priority of issues created without one (none: the middle priority).
ALTER TABLE priorities ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE;

-- Last sign-in with a password (form), for administrators.
ALTER TABLE users ADD COLUMN last_login_at DATETIME(3) NULL;

-- Where a custom field applies: the projects and the issue types listed,
-- all of them when none is.
CREATE TABLE custom_field_projects (
    field_id   BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    PRIMARY KEY (field_id, project_id),
    KEY ix_cfp_project (project_id),
    CONSTRAINT fk_cfp_field FOREIGN KEY (field_id) REFERENCES custom_fields (id) ON DELETE CASCADE,
    CONSTRAINT fk_cfp_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);

CREATE TABLE custom_field_issue_types (
    field_id      BIGINT NOT NULL,
    issue_type_id VARCHAR(60) NOT NULL,
    PRIMARY KEY (field_id, issue_type_id),
    CONSTRAINT fk_cfit_field FOREIGN KEY (field_id) REFERENCES custom_fields (id) ON DELETE CASCADE,
    CONSTRAINT fk_cfit_type FOREIGN KEY (issue_type_id) REFERENCES issue_types (id) ON DELETE CASCADE
);
