-- The columns a filter's results show, for everyone who sees it: a JSON list
-- of column ids, NULL for the default ones. A filter takes the columns of its
-- owner's first results gadget on it that has some.
ALTER TABLE filters ADD COLUMN columns TEXT NULL;

UPDATE filters f
    JOIN dashboard_gadgets g ON g.filter_id = f.id AND g.user_id = f.owner_id
    SET f.columns = JSON_EXTRACT(g.config, '$.columns')
    WHERE f.columns IS NULL AND g.kind = 'results' AND JSON_LENGTH(g.config, '$.columns') > 0
        AND NOT EXISTS (SELECT 1 FROM dashboard_gadgets h
            WHERE h.filter_id = g.filter_id AND h.user_id = g.user_id AND h.kind = 'results'
                AND JSON_LENGTH(h.config, '$.columns') > 0 AND h.position < g.position);
