-- The resolution of issues reaching a done status without one (none: they
-- stay unresolved). It used to be guessed from the names on every
-- transition; the same guess picks it once here.
ALTER TABLE resolutions ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE resolutions SET is_default = TRUE WHERE id = (
    SELECT id FROM (
        SELECT id FROM resolutions
        ORDER BY CASE LOWER(name) WHEN 'done' THEN 1 WHEN 'terminé' THEN 2 WHEN 'fixed' THEN 3 WHEN 'corrigé' THEN 4
            ELSE 5 END, position, name
        LIMIT 1
    ) x
);
