CREATE TABLE users (
    id                   INTEGER PRIMARY KEY,
    username             TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    -- Authentication source: "local" for now, later "ldap" or "oidc".
    source               TEXT    NOT NULL DEFAULT 'local',
    -- Empty for users authenticated by an external provider.
    password_hash        TEXT    NOT NULL DEFAULT '',
    must_change_password INTEGER NOT NULL DEFAULT 0,
    created_at           INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at           INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE sessions (
    -- SHA-256 of the session token; the token itself is never stored.
    id         TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    expires_at INTEGER NOT NULL
);

CREATE INDEX sessions_user_id ON sessions (user_id);
CREATE INDEX sessions_expires_at ON sessions (expires_at);
