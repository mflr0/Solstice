-- Workflows belong to a project: no shared workflow schemes. A project has a
-- default workflow and may give some issue types their own.

CREATE TABLE workflows (
    id                BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id        BIGINT NOT NULL,
    name              VARCHAR(255) NOT NULL,
    is_default        BOOLEAN NOT NULL DEFAULT FALSE,
    initial_status_id VARCHAR(60) NOT NULL,
    KEY ix_workflows_project (project_id),
    CONSTRAINT fk_workflows_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_workflows_initial FOREIGN KEY (initial_status_id) REFERENCES statuses (id)
);

-- Issue types that use another workflow than the project's default.
CREATE TABLE workflow_issue_types (
    project_id    BIGINT NOT NULL,
    issue_type_id VARCHAR(60) NOT NULL,
    workflow_id   BIGINT NOT NULL,
    PRIMARY KEY (project_id, issue_type_id),
    CONSTRAINT fk_wit_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_wit_type FOREIGN KEY (issue_type_id) REFERENCES issue_types (id) ON DELETE CASCADE,
    CONSTRAINT fk_wit_workflow FOREIGN KEY (workflow_id) REFERENCES workflows (id) ON DELETE CASCADE
);

CREATE TABLE workflow_statuses (
    workflow_id BIGINT NOT NULL,
    status_id   VARCHAR(60) NOT NULL,
    position    INT NOT NULL DEFAULT 0,
    PRIMARY KEY (workflow_id, status_id),
    CONSTRAINT fk_ws_workflow FOREIGN KEY (workflow_id) REFERENCES workflows (id) ON DELETE CASCADE,
    CONSTRAINT fk_ws_status FOREIGN KEY (status_id) REFERENCES statuses (id)
);

-- from_status_id NULL: the transition is available from any status.
CREATE TABLE workflow_transitions (
    id             BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    workflow_id    BIGINT NOT NULL,
    name           VARCHAR(255) NOT NULL,
    from_status_id VARCHAR(60) NULL,
    to_status_id   VARCHAR(60) NOT NULL,
    position       INT NOT NULL DEFAULT 0,
    KEY ix_wt_workflow (workflow_id),
    CONSTRAINT fk_wt_workflow FOREIGN KEY (workflow_id) REFERENCES workflows (id) ON DELETE CASCADE,
    CONSTRAINT fk_wt_from FOREIGN KEY (from_status_id) REFERENCES statuses (id),
    CONSTRAINT fk_wt_to FOREIGN KEY (to_status_id) REFERENCES statuses (id)
);
