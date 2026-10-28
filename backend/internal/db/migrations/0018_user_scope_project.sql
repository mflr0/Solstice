-- The project a user chose to scope Work, Boards and Search to. NULL covers
-- every project. Deleting the project brings the user back to every project.

ALTER TABLE users
    ADD COLUMN scope_project_id BIGINT NULL,
    ADD CONSTRAINT fk_users_scope_project FOREIGN KEY (scope_project_id) REFERENCES projects (id) ON DELETE SET NULL;
