// Package store reads and writes the control-plane data in PostgreSQL.
package store

import (
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

// Errors common to every repository.
var (
	ErrNotFound = errors.New("not_found")
	ErrConflict = errors.New("conflict")    // the object already exists
	ErrState    = errors.New("wrong_state") // the transition is not allowed now
)

// Store groups the repositories over one database.
type Store struct{ DB *sql.DB }

// New creates a store.
func New(db *sql.DB) *Store { return &Store{DB: db} }

// translate maps PostgreSQL errors to the package errors.
func translate(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var pe *pq.Error
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23505": // unique_violation
			return ErrConflict
		case "23503": // foreign_key_violation
			return ErrNotFound
		}
	}
	return err
}
