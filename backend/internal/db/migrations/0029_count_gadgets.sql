-- My space has no count view: a count gadget becomes a list of its filter.
-- Its field stays in its config, where nothing reads it.
UPDATE dashboard_gadgets SET kind = 'results' WHERE kind = 'count';
