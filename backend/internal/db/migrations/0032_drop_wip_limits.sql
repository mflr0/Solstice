-- Solstice has no WIP limits: columns lose their minimum and maximum.
ALTER TABLE board_columns DROP COLUMN min_issues, DROP COLUMN max_issues;
