-- Configuration documents of the services, stored as JSON and validated by the
-- application. Keys are namespaced by service, e.g. "ntp".
CREATE TABLE settings (
    key        TEXT    PRIMARY KEY,
    value      TEXT    NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
