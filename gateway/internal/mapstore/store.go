// Package mapstore carica, verifica e tiene attiva la mappa firmata.
package mapstore

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// Store tiene il sito attivo e lo sostituisce in modo atomico.
// Una richiesta usa un solo Site dall'inizio alla fine.
type Store struct {
	path   string
	pub    ed25519.PublicKey
	keyID  string
	active atomic.Pointer[site.Site]
}

// NewStore crea uno store che legge il documento firmato da path.
func NewStore(path string, pub ed25519.PublicKey, keyID string) *Store {
	return &Store{path: path, pub: pub, keyID: keyID}
}

// Current restituisce il sito attivo o nil se nessuna mappa è stata caricata.
func (s *Store) Current() *site.Site {
	return s.active.Load()
}

// Reload legge, verifica, compila e attiva la mappa. In caso di errore il
// sito precedente resta in uso (fail closed solo se non ce n'è nessuno).
func (s *Store) Reload() (*site.Site, error) {
	doc, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("lettura mappa: %w", err)
	}
	m, _, err := Verify(doc, s.pub, s.keyID)
	if err != nil {
		return nil, err
	}
	st, err := site.Build(m)
	if err != nil {
		return nil, fmt.Errorf("mappa non utilizzabile: %w", err)
	}
	s.active.Store(st)
	return st, nil
}
