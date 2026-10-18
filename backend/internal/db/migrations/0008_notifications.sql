-- E-mail notifications and webhooks.

-- Instance settings, one value per name (JSON or text), shared by every
-- feature. Secrets inside are sealed with the instance key (package secret).
CREATE TABLE settings (
    name       VARCHAR(191) NOT NULL PRIMARY KEY,
    value      MEDIUMTEXT NOT NULL,
    updated_at DATETIME(3) NOT NULL
);

-- What each user wants by e-mail. No row: the defaults (immediate, every
-- event, not one's own changes, no due-date reminder).
-- delivery: immediate (batched per issue), digest (once a day at digest_hour), none.
-- disabled_events: event groups turned off, comma-separated (created, assigned…).
-- Hours are in the server's time zone (SOLSTICE_TZ).
CREATE TABLE notification_prefs (
    user_id        BIGINT NOT NULL PRIMARY KEY,
    delivery       VARCHAR(10) NOT NULL DEFAULT 'immediate',
    digest_hour    TINYINT NOT NULL DEFAULT 8,
    own_changes    BOOLEAN NOT NULL DEFAULT FALSE,
    disabled_events VARCHAR(255) NOT NULL DEFAULT '',
    due_reminder   BOOLEAN NOT NULL DEFAULT FALSE,
    reminder_hour  TINYINT NOT NULL DEFAULT 8,
    reminder_sent  DATE NULL,  -- last day a reminder went out
    updated_at     DATETIME(3) NOT NULL,
    CONSTRAINT fk_np_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Per-project overrides, kept simple: notifications on or off, and groups or
-- project roles that also receive some events.
CREATE TABLE project_notifications (
    project_id BIGINT NOT NULL PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT fk_pn_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);

CREATE TABLE project_notification_recipients (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    event      VARCHAR(20) NOT NULL,
    group_id   BIGINT NULL,
    role_id    BIGINT NULL,
    KEY ix_pnr_project (project_id),
    CONSTRAINT fk_pnr_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_pnr_group FOREIGN KEY (group_id) REFERENCES user_groups (id) ON DELETE CASCADE,
    CONSTRAINT fk_pnr_role FOREIGN KEY (role_id) REFERENCES project_roles (id) ON DELETE CASCADE
);

-- Changes waiting to be mailed. An immediate batch gathers one user's
-- changes on one issue for a couple of minutes, so a burst of edits makes a
-- single e-mail; a digest batch (issue_id 0) gathers a user's day.
-- issue_id has no foreign key: a deleted issue is still announced.
CREATE TABLE notification_batches (
    id       BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id  BIGINT NOT NULL,
    digest   BOOLEAN NOT NULL,
    issue_id BIGINT NOT NULL,
    first_at DATETIME(3) NOT NULL,
    send_at  DATETIME(3) NOT NULL,
    UNIQUE KEY uq_nb (user_id, digest, issue_id),
    KEY ix_nb_send (send_at),
    CONSTRAINT fk_nb_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- One change in a batch, described in JSON (what changed, the comment…).
CREATE TABLE notification_items (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    batch_id   BIGINT NOT NULL,
    issue_id   BIGINT NOT NULL,
    data       MEDIUMTEXT NOT NULL,
    created_at DATETIME(3) NOT NULL,
    KEY ix_ni_batch (batch_id),
    CONSTRAINT fk_ni_batch FOREIGN KEY (batch_id) REFERENCES notification_batches (id) ON DELETE CASCADE
);

-- E-mails ready to go. A worker sends them, retrying with growing delays;
-- claim marks the rows a worker is sending (a lease that expires with
-- next_attempt_at if the worker dies).
-- status: pending, sent, failed (gave up).
CREATE TABLE mail_outbox (
    id              BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    kind            VARCHAR(20) NOT NULL,  -- notification, digest, subscription, reminder
    user_id         BIGINT NULL,
    to_addr         VARCHAR(320) NOT NULL,
    subject         VARCHAR(998) NOT NULL,
    text_body       MEDIUMTEXT NOT NULL,
    html_body       MEDIUMTEXT NOT NULL,
    message_id      VARCHAR(255) NOT NULL,
    in_reply_to     VARCHAR(255) NULL,
    status          VARCHAR(10) NOT NULL DEFAULT 'pending',
    attempts        INT NOT NULL DEFAULT 0,
    next_attempt_at DATETIME(3) NOT NULL,
    claim           CHAR(32) NULL,
    last_error      TEXT NULL,
    created_at      DATETIME(3) NOT NULL,
    sent_at         DATETIME(3) NULL,
    KEY ix_mo_due (status, next_attempt_at),
    KEY ix_mo_claim (claim),
    CONSTRAINT fk_mo_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Filter subscriptions: a saved filter's results mailed on a schedule to a
-- user, or to every member of a group.
-- days: ISO weekdays (1 = Monday) for weekly schedules, comma-separated.
CREATE TABLE filter_subscriptions (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    filter_id   BIGINT NOT NULL,
    owner_id    BIGINT NOT NULL,
    user_id     BIGINT NULL,
    group_id    BIGINT NULL,
    frequency   VARCHAR(10) NOT NULL,  -- daily, weekly
    days        VARCHAR(20) NOT NULL DEFAULT '',
    hour        TINYINT NOT NULL,
    empty_too   BOOLEAN NOT NULL DEFAULT FALSE,
    last_run_at DATETIME(3) NULL,
    next_run_at DATETIME(3) NOT NULL,
    created_at  DATETIME(3) NOT NULL,
    KEY ix_fsub_filter (filter_id),
    KEY ix_fsub_next (next_run_at),
    CONSTRAINT fk_fsub_filter FOREIGN KEY (filter_id) REFERENCES filters (id) ON DELETE CASCADE,
    CONSTRAINT fk_fsub_owner FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_fsub_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_fsub_group FOREIGN KEY (group_id) REFERENCES user_groups (id) ON DELETE CASCADE
);

-- Webhooks: Solstice POSTs Jira-shaped JSON to url on the chosen events.
-- events: comma-separated Jira event names (jira:issue_created…).
-- secret: sealed (package secret); signs bodies with HMAC-SHA256.
CREATE TABLE webhooks (
    id           BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name         VARCHAR(255) NOT NULL,
    url          TEXT NOT NULL,
    events       VARCHAR(1000) NOT NULL,
    jql          TEXT NULL,
    secret       TEXT NULL,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    exclude_body BOOLEAN NOT NULL DEFAULT FALSE,
    created_by   BIGINT NULL,
    created_at   DATETIME(3) NOT NULL,
    updated_at   DATETIME(3) NOT NULL,
    CONSTRAINT fk_webhooks_user FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL
);

-- Deliveries, queued then sent with retries; the last ones of each webhook
-- are kept as its delivery log.
-- status: pending, sent, failed (gave up).
CREATE TABLE webhook_deliveries (
    id              BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    webhook_id      BIGINT NOT NULL,
    event           VARCHAR(60) NOT NULL,
    url             TEXT NOT NULL,
    payload         MEDIUMTEXT NOT NULL,
    status          VARCHAR(10) NOT NULL DEFAULT 'pending',
    attempts        INT NOT NULL DEFAULT 0,
    next_attempt_at DATETIME(3) NOT NULL,
    claim           CHAR(32) NULL,
    status_code     INT NULL,
    duration_ms     INT NULL,
    error           TEXT NULL,
    created_at      DATETIME(3) NOT NULL,
    attempted_at    DATETIME(3) NULL,
    KEY ix_wd_due (status, next_attempt_at),
    KEY ix_wd_hook (webhook_id, id),
    KEY ix_wd_claim (claim),
    CONSTRAINT fk_wd_webhook FOREIGN KEY (webhook_id) REFERENCES webhooks (id) ON DELETE CASCADE
);
