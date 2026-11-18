// Package placeholder risolve i segnaposto {{spazio.nome}} di legacy_request.
//
// Spazi ammessi: params (percorso), query, fields (form), account (account
// legacy del ruolo), user (utente autenticato). I valori arrivano già
// validati: questo pacchetto si limita a sostituirli.
package placeholder

import (
	"fmt"
	"strings"
)

// Values contiene i valori per spazio: Values["params"]["id"].
type Values map[string]map[string]string

var namespaces = map[string]bool{"params": true, "query": true, "fields": true, "account": true, "user": true}

// Resolve sostituisce i segnaposto in tpl.
//
// Se tpl è composto da un solo segnaposto e il valore manca, restituisce
// absent=true: chi costruisce la richiesta omette quel parametro. In un
// template misto un valore mancante è un errore, per non costruire valori
// parziali.
func Resolve(tpl string, vals Values) (out string, absent bool, err error) {
	if !strings.Contains(tpl, "{{") {
		if strings.Contains(tpl, "}}") {
			return "", false, fmt.Errorf("segnaposto malformato in %q", tpl)
		}
		return tpl, false, nil
	}
	var b strings.Builder
	rest := tpl
	single := strings.HasPrefix(tpl, "{{") && strings.HasSuffix(tpl, "}}") && strings.Count(tpl, "{{") == 1
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			if strings.Contains(rest, "}}") {
				return "", false, fmt.Errorf("segnaposto malformato in %q", tpl)
			}
			b.WriteString(rest)
			break
		}
		end := strings.Index(rest[start:], "}}")
		if end < 0 {
			return "", false, fmt.Errorf("segnaposto non chiuso in %q", tpl)
		}
		b.WriteString(rest[:start])
		key := strings.TrimSpace(rest[start+2 : start+end])
		ns, name, ok := strings.Cut(key, ".")
		if !ok || !namespaces[ns] || name == "" {
			return "", false, fmt.Errorf("segnaposto non valido %q", key)
		}
		v, found := vals[ns][name]
		if !found {
			if single {
				return "", true, nil
			}
			return "", false, fmt.Errorf("valore mancante per %q", key)
		}
		b.WriteString(v)
		rest = rest[start+end+2:]
	}
	return b.String(), false, nil
}

// Check verifica la sintassi di un template senza valori.
func Check(tpl string) error {
	probe := Values{}
	for ns := range namespaces {
		probe[ns] = map[string]string{}
	}
	_, _, err := Resolve(tpl, probe)
	if err != nil && strings.Contains(err.Error(), "valore mancante") {
		return nil
	}
	return err
}
