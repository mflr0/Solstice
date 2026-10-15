-- Saved searches. The query is JQL or the simple search syntax.

CREATE TABLE filters (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    owner_id    BIGINT NULL,
    name        VARCHAR(255) NOT NULL,
    description TEXT NULL,
    query       TEXT NOT NULL,
    created_at  DATETIME(3) NOT NULL,
    updated_at  DATETIME(3) NOT NULL,
    KEY ix_filters_owner (owner_id),
    CONSTRAINT fk_filters_owner FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE SET NULL
);

-- Who besides the owner sees a filter.
-- kind: everyone, group, project (role_id narrows it to a role), user.
CREATE TABLE filter_shares (
    id         BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    filter_id  BIGINT NOT NULL,
    kind       VARCHAR(20) NOT NULL,
    group_id   BIGINT NULL,
    project_id BIGINT NULL,
    role_id    BIGINT NULL,
    user_id    BIGINT NULL,
    KEY ix_fs_filter (filter_id),
    CONSTRAINT fk_fs_filter FOREIGN KEY (filter_id) REFERENCES filters (id) ON DELETE CASCADE,
    CONSTRAINT fk_fs_group FOREIGN KEY (group_id) REFERENCES user_groups (id) ON DELETE CASCADE,
    CONSTRAINT fk_fs_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_fs_role FOREIGN KEY (role_id) REFERENCES project_roles (id) ON DELETE CASCADE,
    CONSTRAINT fk_fs_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE filter_favourites (
    filter_id BIGINT NOT NULL,
    user_id   BIGINT NOT NULL,
    PRIMARY KEY (filter_id, user_id),
    KEY ix_ff_user (user_id),
    CONSTRAINT fk_ff_filter FOREIGN KEY (filter_id) REFERENCES filters (id) ON DELETE CASCADE,
    CONSTRAINT fk_ff_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
