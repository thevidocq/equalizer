// Package mapmodel definisce i tipi della mappa usati dal gateway.
package mapmodel

// Model è il sottoinsieme della mappa (schema/map/gateway-map.schema.json)
// usato finora dal gateway. La validazione completa contro lo schema avviene
// nel pannello prima della firma; il gateway si fida solo della firma.
//
// TODO: generare questi tipi da schema/ quando la generazione automatica
// sarà disponibile (vedi CHANGELOG).
type Model struct {
	Meta     Meta               `json:"meta"`
	Legacy   Legacy             `json:"legacy"`
	Roles    Roles              `json:"roles"`
	Defaults Defaults           `json:"defaults"`
	Types    map[string]TypeDef `json:"types"`
	Routes   []Route            `json:"routes"`
	Forms    []Form             `json:"forms"`
	API      []APIRoute         `json:"api"`
	Compat   []Compat           `json:"compat"`
	Assets   []Asset            `json:"assets"`
}

// Asset è un prefisso di file statici del legacy servito dal gateway.
type Asset struct {
	Prefix    string   `json:"prefix"`
	Types     []string `json:"types"`
	MaxSizeKB int      `json:"max_size_kb"`
	Roles     []string `json:"roles"`
}

// Meta identifica tenant, app e versione.
type Meta struct {
	Schema  string `json:"schema"`
	Tenant  string `json:"tenant"`
	App     string `json:"app"`
	Version int    `json:"version"`
	Status  string `json:"status"`
}

// Legacy descrive come raggiungere l'applicazione protetta.
type Legacy struct {
	BaseURL   string `json:"base_url"`
	Reach     Reach  `json:"reach"`
	UserAgent string `json:"user_agent"`
	TimeoutMS int    `json:"timeout_ms"`
}

// Reach indica se il legacy si raggiunge via connettore o direttamente.
type Reach struct {
	Mode        string   `json:"mode"`
	ConnectorID string   `json:"connector_id"`
	Allow       []string `json:"allow"`
}

// Roles elenca i ruoli e quello dei visitatori non autenticati.
type Roles struct {
	Anonymous string         `json:"anonymous"`
	List      map[string]any `json:"list"`
}

// Defaults contiene i valori predefiniti della mappa.
type Defaults struct {
	String       StringDefaults `json:"string"`
	LegacyBudget LegacyBudget   `json:"legacy_budget"`
}

// StringDefaults si applica ai campi string senza vincoli propri.
type StringDefaults struct {
	MaxLength int    `json:"max_length"`
	Charset   string `json:"charset"`
}

// LegacyBudget limita il carico verso il legacy.
type LegacyBudget struct {
	MaxConcurrency int     `json:"max_concurrency"`
	MaxRPS         float64 `json:"max_rps"`
}

// TypeDef è un tipo riutilizzabile definito in "types".
type TypeDef struct {
	Base      string   `json:"base"`
	Min       *float64 `json:"min"`
	Max       *float64 `json:"max"`
	Values    []string `json:"values"`
	Pattern   string   `json:"pattern"`
	Charset   string   `json:"charset"`
	MaxLength int      `json:"max_length"`
}

// FieldSpec descrive un valore in ingresso dal client.
type FieldSpec struct {
	Type      string   `json:"type"`
	Required  *bool    `json:"required"`
	Default   any      `json:"default"`
	Min       *float64 `json:"min"`
	Max       *float64 `json:"max"`
	Values    []string `json:"values"`
	Pattern   string   `json:"pattern"`
	Charset   string   `json:"charset"`
	MaxLength int      `json:"max_length"`
	Rules     []string `json:"rules"`
}

// LegacyRequest è la richiesta che il gateway costruisce verso il legacy.
type LegacyRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query"`
	Form    map[string]string `json:"form"`
	Headers map[string]string `json:"headers"`
}

// Route è una pagina in lettura.
type Route struct {
	ID            string               `json:"id"`
	Path          string               `json:"path"`
	Method        string               `json:"method"`
	Roles         []string             `json:"roles"`
	Render        string               `json:"render"`
	Params        map[string]FieldSpec `json:"params"`
	Query         map[string]FieldSpec `json:"query"`
	LegacyRequest LegacyRequest        `json:"legacy_request"`
	Links         []Link               `json:"links"`
}

// Link traduce un URL del legacy trovato in una pagina in una rotta pubblica.
// Formato di Legacy: "/percorso?chiave={nome}", dove {nome} è un parametro
// o un valore di query della rotta di destinazione.
type Link struct {
	Legacy string `json:"legacy"`
	Route  string `json:"route"`
}

// Form è un'azione di scrittura (POST) su un percorso pubblico.
type Form struct {
	ID    string   `json:"id"`
	Route string   `json:"route"`
	Roles []string `json:"roles"`
}

// APIRoute è un endpoint JSON per le parti dinamiche.
type APIRoute struct {
	ID     string   `json:"id"`
	Path   string   `json:"path"`
	Method string   `json:"method"`
	Roles  []string `json:"roles"`
}

// Compat è un URL storico del legacy.
type Compat struct {
	Legacy string `json:"legacy"`
	Action string `json:"action"`
	Route  string `json:"route"`
}

// RouteByID restituisce la rotta con l'id indicato.
func (m *Model) RouteByID(id string) (*Route, bool) {
	for i := range m.Routes {
		if m.Routes[i].ID == id {
			return &m.Routes[i], true
		}
	}
	return nil, false
}
