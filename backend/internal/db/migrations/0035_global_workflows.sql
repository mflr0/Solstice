-- Global workflows belong to no project: project_id NULL. Projects copy
-- them; is_default marks the one global default.
ALTER TABLE workflows MODIFY project_id BIGINT NULL;
