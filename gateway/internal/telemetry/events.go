package telemetry

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Event è un evento di sicurezza conforme a schema/events/event.schema.json.
type Event struct {
	Schema     string         `json:"schema"`
	TS         string         `json:"ts"`
	Type       string         `json:"type"`
	Reason     *string        `json:"reason"`
	Service    string         `json:"service"`
	Instance   string         `json:"instance,omitempty"`
	Tenant     string         `json:"tenant"`
	App        string         `json:"app"`
	MapVersion *int           `json:"map_version"`
	Route      *string        `json:"route"`
	RequestID  string         `json:"request_id,omitempty"`
	SrcIP      string         `json:"src_ip,omitempty"`
	Method     string         `json:"method,omitempty"`
	Path       string         `json:"path,omitempty"`
	Status     int            `json:"status,omitempty"`
	LatencyMS  *Latency       `json:"latency_ms,omitempty"`
	Detail     map[string]any `json:"detail,omitempty"`
}

// Latency divide il tempo tra gateway e legacy.
type Latency struct {
	Gateway float64 `json:"gateway"`
	Legacy  float64 `json:"legacy"`
}

// Emitter scrive eventi come righe JSON (stdout), letti da Vector.
type Emitter struct {
	mu       sync.Mutex
	enc      *json.Encoder
	instance string
	now      func() time.Time
}

// NewEmitter crea un emitter che scrive su w.
func NewEmitter(w io.Writer, instance string) *Emitter {
	return &Emitter{enc: json.NewEncoder(w), instance: instance, now: time.Now}
}

// Emit completa i campi comuni e scrive l'evento.
func (e *Emitter) Emit(ev Event) {
	ev.Schema = "event/1.0"
	ev.Service = ServiceName
	ev.Instance = e.instance
	if ev.TS == "" {
		ev.TS = e.now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
	}
	if len(ev.Path) > 2048 {
		ev.Path = ev.Path[:2048]
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_ = e.enc.Encode(ev)
}

// Ptr restituisce un puntatore al valore, per i campi nullable.
func Ptr[T any](v T) *T { return &v }
