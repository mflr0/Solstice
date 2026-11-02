-- Interface preferences that follow a user across devices: one JSON value
-- per user and name (the names the API knows). No row leaves the default.
CREATE TABLE user_preferences (
    user_id BIGINT NOT NULL,
    name    VARCHAR(40) NOT NULL,
    value   TEXT NOT NULL,
    PRIMARY KEY (user_id, name),
    CONSTRAINT fk_user_preferences_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
