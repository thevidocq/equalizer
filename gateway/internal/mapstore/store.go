package mapstore

import (
	"crypto/ed25519"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/thevidocq/equalizer/gateway/internal/router"
)

// Active è una mappa verificata con il suo router, pronta all'uso.
// Una richiesta usa un solo Active dall'inizio alla fine.
type Active struct {
	Model  *Model
	Router *router.Router
}

// Store tiene la mappa attiva e la sostituisce in modo atomico.
type Store struct {
	path   string
	pub    ed25519.PublicKey
	keyID  string
	active atomic.Pointer[Active]
}

// NewStore crea uno store che legge il documento firmato da path.
func NewStore(path string, pub ed25519.PublicKey, keyID string) *Store {
	return &Store{path: path, pub: pub, keyID: keyID}
}

// Current restituisce la mappa attiva o nil se nessuna è stata caricata.
func (s *Store) Current() *Active {
	return s.active.Load()
}

// Reload legge, verifica e attiva la mappa. In caso di errore la mappa
// attiva precedente resta in uso (fail closed solo se non ce n'è nessuna).
func (s *Store) Reload() (*Active, error) {
	doc, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("lettura mappa: %w", err)
	}
	m, _, err := Verify(doc, s.pub, s.keyID)
	if err != nil {
		return nil, err
	}
	rt, err := router.New(Entries(m))
	if err != nil {
		return nil, fmt.Errorf("compilazione rotte: %w", err)
	}
	a := &Active{Model: m, Router: rt}
	s.active.Store(a)
	return a, nil
}

// Entries converte rotte, form e API della mappa in voci per il router.
func Entries(m *Model) []router.Entry {
	var out []router.Entry
	for _, r := range m.Routes {
		out = append(out, router.Entry{ID: r.ID, Kind: router.KindRoute, Method: http.MethodGet, Pattern: r.Path})
	}
	for _, f := range m.Forms {
		out = append(out, router.Entry{ID: f.ID, Kind: router.KindForm, Method: http.MethodPost, Pattern: f.Route})
	}
	for _, a := range m.API {
		out = append(out, router.Entry{ID: a.ID, Kind: router.KindAPI, Method: a.Method, Pattern: a.Path})
	}
	return out
}
