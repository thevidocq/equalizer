// Package mapstore carica, verifica e tiene attiva la mappa firmata.
package mapstore

// Model è il sottoinsieme della mappa (schema/map/gateway-map.schema.json)
// usato finora dal gateway. La validazione completa contro lo schema avviene
// nel pannello prima della firma; il gateway si fida solo della firma.
//
// TODO: generare questi tipi da schema/ quando la generazione automatica
// sarà disponibile (vedi CHANGELOG).
type Model struct {
	Meta   Meta       `json:"meta"`
	Legacy Legacy     `json:"legacy"`
	Routes []Route    `json:"routes"`
	Forms  []Form     `json:"forms"`
	API    []APIRoute `json:"api"`
	Compat []Compat   `json:"compat"`
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
	BaseURL string `json:"base_url"`
	Reach   Reach  `json:"reach"`
}

// Reach indica se il legacy si raggiunge via connettore o direttamente.
type Reach struct {
	Mode        string   `json:"mode"`
	ConnectorID string   `json:"connector_id"`
	Allow       []string `json:"allow"`
}

// Route è una pagina in lettura.
type Route struct {
	ID     string   `json:"id"`
	Path   string   `json:"path"`
	Method string   `json:"method"`
	Roles  []string `json:"roles"`
	Render string   `json:"render"`
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
