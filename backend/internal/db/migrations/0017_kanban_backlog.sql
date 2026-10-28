-- Kanban boards may keep their first column's issues on a backlog page
-- instead of on the board (Jira's "Kanban backlog").
ALTER TABLE boards
    ADD COLUMN kanban_backlog BOOLEAN NOT NULL DEFAULT FALSE;
