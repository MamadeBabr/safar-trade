package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hossen/safar-trade/db"
	"github.com/hossen/safar-trade/internal/model"
)

// NoteRepository defines the contract for persisting and retrieving notes.
//
// Node comparison:
// In TypeScript you write `interface NoteRepository { ... }` and `class PostgresNoteRepo implements NoteRepository`.
// In Go, interfaces are satisfied implicitly without any "implements" keyword.
// If a struct has all the matching methods with identical signatures, it satisfies the interface!
type NoteRepository interface {
	Create(ctx context.Context, note *model.Note) (*model.Note, error)
	GetByID(ctx context.Context, id int64) (*model.Note, error)
	List(ctx context.Context, limit, offset int32) ([]model.Note, error)
	Update(ctx context.Context, note *model.Note) (*model.Note, error)
	Delete(ctx context.Context, id int64) error
}

// pgNoteRepository is the concrete PostgreSQL implementation using sqlc generated queries.
type pgNoteRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// Compile-time interface verification:
// This line fails compilation if *pgNoteRepository ever misses a method from NoteRepository.
var _ NoteRepository = (*pgNoteRepository)(nil)

// NewNoteRepository constructs a new PostgreSQL note repository.
// In Go, constructor functions are named New or New<TypeName>.
func NewNoteRepository(pool *pgxpool.Pool) NoteRepository {
	return &pgNoteRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (r *pgNoteRepository) Create(ctx context.Context, note *model.Note) (*model.Note, error) {
	// context.Context is passed through to every database call.
	// If the client disconnects or an HTTP timeout occurs, pgx automatically cancels the DB query!
	res, err := r.queries.CreateNote(ctx, db.CreateNoteParams{
		UserID:   note.UserID,
		Title:    note.Title,
		Content:  note.Content,
		Category: note.Category,
	})
	if err != nil {
		return nil, err
	}

	return toDomainNote(res), nil
}

func (r *pgNoteRepository) GetByID(ctx context.Context, id int64) (*model.Note, error) {
	res, err := r.queries.GetNoteByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Translate driver-specific error to domain sentinel error
			return nil, ErrNotFound
		}
		return nil, err
	}

	return toDomainNote(res), nil
}

func (r *pgNoteRepository) List(ctx context.Context, limit, offset int32) ([]model.Note, error) {
	rows, err := r.queries.ListNotes(ctx, db.ListNotesParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	notes := make([]model.Note, len(rows))
	for i, row := range rows {
		notes[i] = *toDomainNote(row)
	}
	return notes, nil
}

func (r *pgNoteRepository) Update(ctx context.Context, note *model.Note) (*model.Note, error) {
	res, err := r.queries.UpdateNote(ctx, db.UpdateNoteParams{
		ID:       note.ID,
		Title:    note.Title,
		Content:  note.Content,
		Category: note.Category,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return toDomainNote(res), nil
}

func (r *pgNoteRepository) Delete(ctx context.Context, id int64) error {
	// First check if note exists to return a 404 if trying to delete a non-existent note
	_, err := r.queries.GetNoteByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	return r.queries.DeleteNote(ctx, id)
}

// toDomainNote maps an sqlc-generated db.Note to our domain model.Note.
func toDomainNote(n db.Note) *model.Note {
	return &model.Note{
		ID:        n.ID,
		UserID:    n.UserID,
		Title:     n.Title,
		Content:   n.Content,
		Category:  n.Category,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}
