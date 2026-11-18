// Package site compila una versione della mappa in tutto ciò che serve a
// servirla: router, validatore, client verso il legacy, budget.
// Un Site è immutabile: una nuova mappa produce un nuovo Site.
package site

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/budget"
	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/legacyurl"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline/clean"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline/rewrite"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/transport"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// ErrConnectorUnavailable è restituito finché il trasporto via connettore
// non è implementato (arriva con tunnel-relay).
var ErrConnectorUnavailable = errors.New("trasporto via connettore non ancora disponibile")

// Budget di attesa quando il legacy è saturo.
const budgetWait = 2 * time.Second

// Site è una mappa compilata e pronta a servire richieste.
type Site struct {
	Model     *mapmodel.Model
	Router    *router.Router
	Validator *validate.Validator
	Legacy    *legacyclient.Client
	Budget    *budget.Limiter
	URLs      *legacyurl.Rewriter
	Pipeline  *pipeline.Pipeline
}

// DefaultSteps è l'ordine predefinito dei passi della resa fedele.
func DefaultSteps() []pipeline.Step {
	return []pipeline.Step{clean.Step{}, rewrite.Step{}}
}

// Build compila la mappa. Rifiuta mappe incoerenti invece di servirle a metà.
func Build(m *mapmodel.Model) (*Site, error) {
	rt, err := router.New(Entries(m))
	if err != nil {
		return nil, fmt.Errorf("rotte: %w", err)
	}
	if m.Roles.Anonymous != "" {
		if _, ok := m.Roles.List[m.Roles.Anonymous]; !ok {
			return nil, fmt.Errorf("roles.anonymous %q non è in roles.list", m.Roles.Anonymous)
		}
	}

	v := validate.New(m.Types, m.Defaults.String)
	for _, r := range m.Routes {
		if err := v.Check(r.Params); err != nil {
			return nil, fmt.Errorf("rotta %q, params: %w", r.ID, err)
		}
		if err := v.Check(r.Query); err != nil {
			return nil, fmt.Errorf("rotta %q, query: %w", r.ID, err)
		}
		if err := checkTemplates(r.LegacyRequest); err != nil {
			return nil, fmt.Errorf("rotta %q: %w", r.ID, err)
		}
	}

	timeout := time.Duration(m.Legacy.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	var tr http.RoundTripper
	switch m.Legacy.Reach.Mode {
	case "direct":
		tr, err = transport.Direct(m.Legacy.Reach.Allow, timeout)
		if err != nil {
			return nil, fmt.Errorf("legacy.reach: %w", err)
		}
	case "connector":
		tr = unavailable{}
	default:
		return nil, fmt.Errorf("legacy.reach.mode sconosciuto: %q", m.Legacy.Reach.Mode)
	}
	lc, err := legacyclient.New(m.Legacy.BaseURL, tr, m.Legacy.UserAgent, timeout)
	if err != nil {
		return nil, err
	}

	urls, err := legacyurl.New(lc.Base(), m)
	if err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}

	return &Site{
		Model:     m,
		Router:    rt,
		Validator: v,
		Legacy:    lc,
		Budget:    budget.New(m.Defaults.LegacyBudget.MaxConcurrency, budgetWait),
		URLs:      urls,
		Pipeline:  pipeline.New(DefaultSteps()...),
	}, nil
}

// AssetByPrefix restituisce la voce asset con quel prefisso.
func (s *Site) AssetByPrefix(prefix string) (*mapmodel.Asset, bool) {
	for i := range s.Model.Assets {
		if s.Model.Assets[i].Prefix == prefix {
			return &s.Model.Assets[i], true
		}
	}
	return nil, false
}

// RoleAllowed dice se il ruolo può accedere a una voce con quei ruoli.
func RoleAllowed(role string, allowed []string) bool {
	return role != "" && slices.Contains(allowed, role)
}

// Entries converte rotte, form e API della mappa in voci per il router.
func Entries(m *mapmodel.Model) []router.Entry {
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
	for _, a := range m.Assets {
		out = append(out, router.Entry{ID: AssetID(a.Prefix), Kind: router.KindAsset, Method: http.MethodGet, Pattern: a.Prefix})
	}
	return out
}

// AssetID è l'id di instradamento di un prefisso di asset.
func AssetID(prefix string) string { return "asset:" + prefix }

func checkTemplates(lr mapmodel.LegacyRequest) error {
	if err := placeholder.Check(lr.Path); err != nil {
		return fmt.Errorf("legacy_request.path: %w", err)
	}
	for _, group := range []map[string]string{lr.Query, lr.Form, lr.Headers} {
		for k, tpl := range group {
			if err := placeholder.Check(tpl); err != nil {
				return fmt.Errorf("legacy_request %q: %w", k, err)
			}
		}
	}
	return nil
}

type unavailable struct{}

func (unavailable) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, ErrConnectorUnavailable
}
