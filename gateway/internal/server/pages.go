package server

import (
	"net/http"
)

// Pagine generiche: non rivelano nulla del legacy né del gateway.
const (
	pageNotFound    = "Pagina non trovata"
	pageNotAllowed  = "Operazione non consentita"
	pageUnavailable = "Servizio temporaneamente non disponibile. Riprova tra poco."
	pageError       = "Si è verificato un errore. Riprova tra poco."
)

func writePage(w http.ResponseWriter, status int, message string) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<!doctype html><html lang="it"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1"><title>` + message + `</title>` +
		`<style>body{font-family:system-ui,sans-serif;margin:15vh auto;max-width:32rem;padding:0 1rem;color:#333}</style>` +
		`</head><body><p>` + message + `</p></body></html>`))
}
