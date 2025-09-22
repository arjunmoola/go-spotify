-- name: InsertConfig :exec
INSERT INTO config (client_secret, client_id, redirect_uri, authorized, access_token, refresh_token, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetClientInfo :one
SELECT client_secret, client_id, redirect_uri, authorized, access_token, refresh_token, expires_at FROM config WHERE id = 1;

-- name: UpdateTokens :exec
UPDATE config
SET
    access_token = ?,
    refresh_token = ?,
    expires_at = ?,
    authorized = 1
WHERE
    id = 1;

