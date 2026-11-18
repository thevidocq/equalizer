// eqtool: strumenti da riga di comando per il prototipo.
//
//	eqtool keygen -out secrets/map-signing.key
//	    crea la coppia di chiavi Ed25519 (file .key privato e .pub pubblico, base64)
//	eqtool sign -key secrets/map-signing.key -map maps/mappa.json -key-id k1 -out maps/current.json
//	    firma una mappa JSON e produce il documento SignedMap letto dal gateway
//
// Nel prodotto finale la firma la fa panel-api; eqtool serve finché il
// pannello non esiste.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(os.Args[2:])
	case "sign":
		err = sign(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "eqtool:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "uso: eqtool keygen -out FILE | eqtool sign -key FILE -map FILE -key-id ID -out FILE")
}

func keygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	out := fs.String("out", "map-signing.key", "file della chiave privata (la pubblica va in .pub)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("%s esiste già: non la sovrascrivo", *out)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, []byte(base64.StdEncoding.EncodeToString(priv.Seed())+"\n"), 0o600); err != nil {
		return err
	}
	pubPath := strings.TrimSuffix(*out, ".key") + ".pub"
	if err := os.WriteFile(pubPath, []byte(base64.StdEncoding.EncodeToString(pub)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("chiave privata: %s\nchiave pubblica: %s\n", *out, pubPath)
	return nil
}

func sign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	keyPath := fs.String("key", "", "chiave privata creata con keygen")
	mapPath := fs.String("map", "", "mappa in JSON (vedi scripts/map_to_json.py)")
	keyID := fs.String("key-id", "k1", "identificativo della chiave")
	out := fs.String("out", "current.json", "documento firmato in uscita")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *keyPath == "" || *mapPath == "" {
		return errors.New("servono -key e -map")
	}
	seedB64, err := os.ReadFile(*keyPath)
	if err != nil {
		return err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(seedB64)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return errors.New("chiave privata non valida")
	}
	priv := ed25519.NewKeyFromSeed(seed)

	raw, err := os.ReadFile(*mapPath)
	if err != nil {
		return err
	}
	var payload bytes.Buffer
	if err := json.Compact(&payload, raw); err != nil {
		return fmt.Errorf("la mappa non è JSON valido: %w", err)
	}
	var meta struct {
		Meta struct {
			Version int    `json:"version"`
			Status  string `json:"status"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(payload.Bytes(), &meta); err != nil {
		return err
	}
	if meta.Meta.Status != "published" {
		return fmt.Errorf("meta.status è %q: il gateway accetta solo mappe published", meta.Meta.Status)
	}

	doc, err := json.MarshalIndent(map[string]any{
		"version":   meta.Meta.Version,
		"payload":   base64.StdEncoding.EncodeToString(payload.Bytes()),
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload.Bytes())),
		"key_id":    *keyID,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, append(doc, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("mappa versione %d firmata: %s\n", meta.Meta.Version, *out)
	return nil
}
