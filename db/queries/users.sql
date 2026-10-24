-- db/queries/users.sql

-- name: CreateUser :one
INSERT INTO users (
    email,
    name
) VALUES (
    $1, $2
)
RETURNING id, email, name, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, name, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, name, created_at, updated_at
FROM users
WHERE email = $1;
