-- name: InsertConfig :exec
INSERT INTO config (client_secret, client_id) VALUES (?, ?);

-- name: GetClientInfo :one
SELECT client_secret, client_id FROM config WHERE id = 1;
