-- db/queries/notes.sql
-- Node comparison: In Node/Prisma you write Prisma schema or Drizzle query builders.
-- In Go with sqlc, you write raw, optimized SQL with type annotations, and sqlc
-- generates compile-time type-safe Go structs and query methods for pgx.

-- name: CreateNote :one
INSERT INTO notes (
    user_id,
    title,
    content,
    category
) VALUES (
    $1, $2, $3, $4
)
RETURNING id, user_id, title, content, category, created_at, updated_at;

-- name: GetNoteByID :one
SELECT id, user_id, title, content, category, created_at, updated_at
FROM notes
WHERE id = $1;

-- name: ListNotes :many
SELECT id, user_id, title, content, category, created_at, updated_at
FROM notes
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateNote :one
UPDATE notes
SET title = $2,
    content = $3,
    category = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING id, user_id, title, content, category, created_at, updated_at;

-- name: DeleteNote :exec
DELETE FROM notes
WHERE id = $1;
