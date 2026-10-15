-- The change log by field: history queries (JQL WAS, CHANGED) read one
-- field's changes across all issues.

CREATE INDEX ix_ci_field ON change_items (field, group_id);
