-- One immediate batch per user for a bulk change (one request changing
-- several issues): bulk_id names the request, issue_id is 0. Other batches
-- keep bulk_id 0.
ALTER TABLE notification_batches
    ADD COLUMN bulk_id BIGINT NOT NULL DEFAULT 0,
    DROP KEY uq_nb,
    ADD UNIQUE KEY uq_nb (user_id, digest, issue_id, bulk_id);
