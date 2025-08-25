CREATE TABLE IF NOT EXISTS config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    client_secret VARCHAR NOT NULL,
    client_id VARCHAR NOT NULL
);
