-- Personal dashboards. A row in dashboards marks one the user has edited;
-- without it the user sees the default gadgets.
CREATE TABLE dashboards (
    user_id    BIGINT NOT NULL PRIMARY KEY,
    updated_at DATETIME(3) NOT NULL,
    CONSTRAINT fk_dashboards_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- kind: results or count (on filter_id), assigned, reported or recent (built
-- in). filter_id turns NULL when the filter is deleted: the gadget stays and
-- says so. config is JSON (columns, size, field).
CREATE TABLE dashboard_gadgets (
    id        BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id   BIGINT NOT NULL,
    position  INT NOT NULL,
    kind      VARCHAR(20) NOT NULL,
    filter_id BIGINT NULL,
    config    TEXT NOT NULL,
    KEY ix_dg_user (user_id, position),
    CONSTRAINT fk_dg_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_dg_filter FOREIGN KEY (filter_id) REFERENCES filters (id) ON DELETE SET NULL
);
