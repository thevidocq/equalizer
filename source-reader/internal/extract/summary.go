// Package extract reads a site's code files and writes what they say as a
// source summary (schema/source/summary.schema.json): entry points, input
// types, risky calls, components. Patterns, not a PHP parser: what it
// cannot read it leaves out, and nothing of the code goes in the summary.
package extract

import "time"

// Summary is schema/source/summary.schema.json.
type Summary struct {
	Schema     string      `json:"schema"`
	ReadAt     time.Time   `json:"read_at"`
	Stack      string      `json:"stack"`
	Files      Files       `json:"files"`
	Routes     []Route     `json:"routes"`
	Inputs     []Input     `json:"inputs"`
	Risks      []Risk      `json:"risks"`
	Components []Component `json:"components"`
	sawPHP     bool
}

// Files counts what was read.
type Files struct {
	Read      int  `json:"read"`
	Skipped   int  `json:"skipped"`
	Truncated bool `json:"truncated,omitempty"`
}

// Where is a place in the code: file relative to the site, line.
type Where struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// Route is an entry point the code declares.
type Route struct {
	Path    string            `json:"path"`
	Methods []string          `json:"methods"`
	Origin  string            `json:"origin"`
	Action  string            `json:"action,omitempty"`
	Public  bool              `json:"public,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
	Where   Where             `json:"where"`
}

// Input is a request key the code reads, and what it makes of it.
type Input struct {
	Name   string   `json:"name"`
	Source string   `json:"source"`
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
	Route  string   `json:"route,omitempty"`
	Where  Where    `json:"where"`
}

// Risk is a call where a request value may do harm.
type Risk struct {
	Kind  string `json:"kind"`
	Input string `json:"input,omitempty"`
	Route string `json:"route,omitempty"`
	Where Where  `json:"where"`
}

// Component is the core, a plugin, a theme or a library, and its version.
type Component struct {
	Name    string  `json:"name"`
	Kind    string  `json:"kind"`
	Version *string `json:"version"`
}

// Caps of the summary's lists, as in the schema.
const (
	maxRoutes     = 2000
	maxInputs     = 5000
	maxRisks      = 2000
	maxComponents = 500
)
