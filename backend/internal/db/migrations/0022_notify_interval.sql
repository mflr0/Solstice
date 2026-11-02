-- delivery may also be periodic: one e-mail every digest_interval minutes
-- (15–720), counted from the first change waiting.
ALTER TABLE notification_prefs ADD COLUMN digest_interval SMALLINT NOT NULL DEFAULT 60 AFTER digest_hour;
