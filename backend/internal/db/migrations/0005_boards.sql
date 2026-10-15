-- Boards (Kanban and Scrum) and sprints.
-- A board shows the issues of a saved filter, laid out in columns of statuses.

ALTER TABLE issues
    ADD COLUMN rank_key VARCHAR(255) COLLATE utf8mb4_bin NULL,  -- board order: smaller first (LexoRank strings)
    ADD COLUMN epic_id  BIGINT NULL,
    ADD KEY ix_issues_rank (rank_key),
    ADD KEY ix_issues_epic (epic_id),
    ADD CONSTRAINT fk_issues_epic FOREIGN KEY (epic_id) REFERENCES issues (id) ON DELETE SET NULL;

CREATE TABLE boards (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    kind        ENUM('kanban', 'scrum') NOT NULL,
    filter_id   BIGINT NOT NULL,          -- the board shows this filter's issues and is visible like it
    sub_query   TEXT NULL,                -- Kanban: which of the filter's issues are on the board
    owner_id    BIGINT NULL,
    swimlanes   VARCHAR(20) NOT NULL DEFAULT 'none',  -- none, custom, assignee, epic, parent
    estimate    VARCHAR(60) NULL,         -- what a card's estimate is: customfield_<id> or original_estimate
    done_cutoff VARCHAR(20) NULL,         -- Kanban: done issues older than this leave the board (e.g. 2w)
    created_at  DATETIME(3) NOT NULL,
    updated_at  DATETIME(3) NOT NULL,
    KEY ix_boards_filter (filter_id),
    CONSTRAINT fk_boards_filter FOREIGN KEY (filter_id) REFERENCES filters (id),
    CONSTRAINT fk_boards_owner FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE SET NULL
);

-- Projects a board belongs to (read from its filter), to list it with them.
CREATE TABLE board_projects (
    board_id   BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    PRIMARY KEY (board_id, project_id),
    KEY ix_bp_project (project_id),
    CONSTRAINT fk_bp_board FOREIGN KEY (board_id) REFERENCES boards (id) ON DELETE CASCADE,
    CONSTRAINT fk_bp_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);

CREATE TABLE board_columns (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    board_id    BIGINT NOT NULL,
    name        VARCHAR(255) NOT NULL,
    position    INT NOT NULL DEFAULT 0,
    min_issues  INT NULL,
    max_issues  INT NULL,
    KEY ix_bc_board (board_id),
    CONSTRAINT fk_bc_board FOREIGN KEY (board_id) REFERENCES boards (id) ON DELETE CASCADE
);

CREATE TABLE board_column_statuses (
    column_id BIGINT NOT NULL,
    status_id VARCHAR(60) NOT NULL,
    position  INT NOT NULL DEFAULT 0,
    PRIMARY KEY (column_id, status_id),
    CONSTRAINT fk_bcs_column FOREIGN KEY (column_id) REFERENCES board_columns (id) ON DELETE CASCADE,
    CONSTRAINT fk_bcs_status FOREIGN KEY (status_id) REFERENCES statuses (id)
);

CREATE TABLE board_quick_filters (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    board_id    BIGINT NOT NULL,
    name        VARCHAR(255) NOT NULL,
    query       TEXT NOT NULL,
    description VARCHAR(255) NULL,
    position    INT NOT NULL DEFAULT 0,
    KEY ix_bqf_board (board_id),
    CONSTRAINT fk_bqf_board FOREIGN KEY (board_id) REFERENCES boards (id) ON DELETE CASCADE
);

-- Custom swimlanes (boards with swimlanes = 'custom'): the first lane whose
-- query matches an issue holds it; the default lane takes the rest.
CREATE TABLE board_swimlanes (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    board_id   BIGINT NOT NULL,
    name       VARCHAR(255) NOT NULL,
    query      TEXT NULL,
    position   INT NOT NULL DEFAULT 0,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    KEY ix_bs_board (board_id),
    CONSTRAINT fk_bs_board FOREIGN KEY (board_id) REFERENCES boards (id) ON DELETE CASCADE
);

CREATE TABLE sprints (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    board_id     BIGINT NULL,
    name         VARCHAR(255) NOT NULL,
    goal         TEXT NULL,
    state        ENUM('future', 'active', 'closed') NOT NULL,
    start_at     DATETIME(3) NULL,
    end_at       DATETIME(3) NULL,
    completed_at DATETIME(3) NULL,
    position     BIGINT NOT NULL DEFAULT 0,
    KEY ix_sprints_board (board_id, state),
    CONSTRAINT fk_sprints_board FOREIGN KEY (board_id) REFERENCES boards (id) ON DELETE SET NULL
);

-- Every sprint an issue has been in. An issue is in at most one open
-- (future or active) sprint; closed ones are its history.
CREATE TABLE sprint_issues (
    sprint_id BIGINT NOT NULL,
    issue_id  BIGINT NOT NULL,
    PRIMARY KEY (sprint_id, issue_id),
    KEY ix_si_issue (issue_id),
    CONSTRAINT fk_si_sprint FOREIGN KEY (sprint_id) REFERENCES sprints (id) ON DELETE CASCADE,
    CONSTRAINT fk_si_issue FOREIGN KEY (issue_id) REFERENCES issues (id) ON DELETE CASCADE
);
