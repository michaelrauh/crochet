-- name: CreateItem :one
INSERT INTO items (name)
VALUES ($1)
RETURNING id, name;

-- name: GetItemByID :one
SELECT id, name
  FROM items
 WHERE id = $1;