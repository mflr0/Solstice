-- How an issue type is drawn: a colour (#rrggbb), a fill (filled, subtle or
-- outline) and an icon from the list in the API; NULL leaves the default.
-- Types still named as a seeded default, in any language, get its icon.
ALTER TABLE issue_types
    ADD COLUMN color VARCHAR(7) NULL,
    ADD COLUMN fill  VARCHAR(20) NULL,
    ADD COLUMN icon  VARCHAR(30) NULL;

UPDATE issue_types SET icon = 'task' WHERE icon IS NULL AND name COLLATE utf8mb4_bin IN ('Task', 'Tâche', 'Aufgabe', 'टास्क');
UPDATE issue_types SET icon = 'bug' WHERE icon IS NULL AND name COLLATE utf8mb4_bin IN ('Bug', 'Fehler', 'बग');
UPDATE issue_types SET icon = 'story' WHERE icon IS NULL AND name COLLATE utf8mb4_bin IN ('Story', 'स्टोरी');
UPDATE issue_types SET icon = 'epic' WHERE icon IS NULL AND name COLLATE utf8mb4_bin IN ('Epic', 'Épopée', 'एपिक');
UPDATE issue_types SET icon = 'subtask' WHERE icon IS NULL AND name COLLATE utf8mb4_bin IN ('Sub-task', 'Sous-tâche', 'Unteraufgabe', 'Sotto-task', 'सब-टास्क');
