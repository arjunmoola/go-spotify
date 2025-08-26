CREATE TABLE IF NOT EXISTS config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    client_secret VARCHAR NOT NULL,
    client_id VARCHAR NOT NULL,
    redirect_uri VARCHAR NOT NULL,
    authorized BOOLEAN NOT NULL DEFAULT 0,
    access_token VARCHAR,
    refresh_token VARCHAR,
    expires_at VARCHAR
);
