package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

type discoveryStart struct {
	UseAI    *bool `json:"use_ai"`
	MaxPages int   `json:"max_pages"`
	Account  *struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"account"`
	Accounts    []testAccount `json:"accounts"`
	Mode        string        `json:"mode"`
	ConnectorID string        `json:"connector_id"`
	Address     string        `json:"address"`
	Scheme      string        `json:"scheme"`
}

// testAccount is one of the test accounts of a discovery.
type testAccount struct {
	Label    string `json:"label"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// maxTestAccounts bounds the logged-in passes of one discovery.
const maxTestAccounts = 3

// runReach is how the agent reaches the site: the published map's, or the
// one given with the first discovery.
type runReach struct {
	BaseURL string          `json:"base_url"`
	Reach   json.RawMessage `json:"reach"`
}

func (s *Server) startDiscovery(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var in discoveryStart
	if !decode(w, r, &in) {
		return
	}
	if in.MaxPages == 0 {
		in.MaxPages = 500
	}
	if in.MaxPages < 10 || in.MaxPages > 2000 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "max_pages: between 10 and 2000")
		return
	}
	reach, err := s.discoveryReach(r, a, in)
	if err != nil {
		s.reachFail(w, r, err, http.StatusUnprocessableEntity)
		return
	}
	n := store.NewDiscovery{App: a.ID, Trigger: "manual", MaxPages: in.MaxPages, Reach: reach, RequestedBy: &u.ID,
		UseAI: (in.UseAI == nil || *in.UseAI) && s.hasLLM(r, a.Tenant)}
	accounts, msg := testAccounts(in)
	if msg != "" {
		fail(w, http.StatusUnprocessableEntity, "invalid", msg)
		return
	}
	if len(accounts) > 0 {
		blob, _ := json.Marshal(accounts)
		n.AccountUser = &accounts[0].Username
		n.AccountsSecret = s.MFA.SealFor("discovery:"+a.ID, blob)
	}
	d, err := s.Store.StartDiscovery(r.Context(), n)
	if errors.Is(err, store.ErrConflict) {
		fail(w, http.StatusConflict, "discovery_running", "a discovery of this site is already queued or running")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "discovery.start", a.ID,
		map[string]any{"id": d.ID, "use_ai": d.UseAI, "with_account": d.WithAccount})
	writeJSON(w, http.StatusCreated, d)
}

// testAccounts gathers account and accounts into one checked list; a
// message when they are not acceptable.
func testAccounts(in discoveryStart) ([]testAccount, string) {
	list := in.Accounts
	if in.Account != nil && in.Account.Username != "" {
		list = append([]testAccount{{Label: "account", Username: in.Account.Username, Password: in.Account.Password}}, list...)
	}
	out := []testAccount{}
	labels := map[string]bool{}
	for i, a := range list {
		a.Username, a.Label = strings.TrimSpace(a.Username), strings.TrimSpace(a.Label)
		if a.Username == "" {
			continue
		}
		if a.Label == "" {
			a.Label = "account " + strconv.Itoa(i+1)
		}
		switch {
		case len(a.Username) > 200 || len(a.Password) > 200:
			return nil, "account: at most 200 characters"
		case len(a.Label) > 60:
			return nil, "accounts: a label has at most 60 characters"
		case labels[a.Label]:
			return nil, "accounts: two accounts have the same label"
		}
		labels[a.Label] = true
		out = append(out, a)
	}
	if len(out) > maxTestAccounts {
		return nil, "accounts: at most 3"
	}
	return out, ""
}

func (s *Server) hasLLM(r *http.Request, tenant string) bool {
	_, err := s.Store.TenantLLM(r.Context(), tenant)
	return err == nil
}

// discoveryReach uses the published map when there is one, otherwise the
// connector and address given, with the same checks as a map.
func (s *Server) discoveryReach(r *http.Request, a store.App, in discoveryStart) (json.RawMessage, error) {
	if v, err := s.Store.Published(r.Context(), a.ID); err == nil {
		var doc struct {
			Legacy struct {
				BaseURL string          `json:"base_url"`
				Reach   json.RawMessage `json:"reach"`
			} `json:"legacy"`
		}
		if err := json.Unmarshal(v.Document, &doc); err != nil {
			return nil, err
		}
		if err := s.checkReach(r.Context(), a.Tenant, v.Document); err != nil {
			return nil, err
		}
		return json.Marshal(runReach{BaseURL: doc.Legacy.BaseURL, Reach: doc.Legacy.Reach})
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if in.Scheme == "" {
		in.Scheme = "https"
	}
	if in.Scheme != "https" && in.Scheme != "http" {
		return nil, &reachError{"invalid", "scheme: https or http"}
	}
	reach, err := newReach(in.Mode, in.ConnectorID, in.Address)
	if err != nil {
		return nil, &reachError{"invalid", "no published map yet: " + err.Error()}
	}
	probe, _ := json.Marshal(map[string]any{"legacy": map[string]any{"reach": json.RawMessage(reach)}})
	if err := s.checkReach(r.Context(), a.Tenant, probe); err != nil {
		return nil, err
	}
	return json.Marshal(runReach{BaseURL: in.Scheme + "://" + a.PublicDomain, Reach: reach})
}

func (s *Server) listDiscoveries(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	ds, err := s.Store.Discoveries(r.Context(), a.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list(ds))
}

func (s *Server) getDiscovery(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	d, err := s.Store.Discovery(r.Context(), a.ID, id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}
