// eqtool: command-line tools for the prototype.
//
//	eqtool keygen -out secrets/map-signing.key
//	    create the Ed25519 key pair (private .key and public .pub files, base64)
//	eqtool formkey -out secrets/form.key
//	    create the key that seals form tokens (shared by all gateway replicas)
//	eqtool sign -key secrets/map-signing.key -map maps/map.json -key-id k1 -out maps/current.json
//	    sign a JSON map and produce the SignedMap document read by the gateway
//
// In the final product panel-api does the signing; eqtool is needed until
// the panel exists.
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
	case "formkey":
		err = formkey(os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "usage: eqtool keygen -out FILE | eqtool formkey -out FILE | eqtool sign -key FILE -map FILE -key-id ID -out FILE")
}

func keygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	out := fs.String("out", "map-signing.key", "private key file (the public key goes to .pub)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("%s already exists: not overwriting it", *out)
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
	fmt.Printf("private key: %s\npublic key: %s\n", *out, pubPath)
	return nil
}

func formkey(args []string) error {
	fs := flag.NewFlagSet("formkey", flag.ContinueOnError)
	out := fs.String("out", "form.key", "form key file (32 random bytes, base64)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("%s already exists: not overwriting it", *out)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	if err := os.WriteFile(*out, []byte(base64.StdEncoding.EncodeToString(key)+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("form key: %s\n", *out)
	return nil
}

func sign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	keyPath := fs.String("key", "", "private key created with keygen")
	mapPath := fs.String("map", "", "map in JSON (see scripts/map_to_json.py)")
	keyID := fs.String("key-id", "k1", "key identifier")
	out := fs.String("out", "current.json", "signed output document")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *keyPath == "" || *mapPath == "" {
		return errors.New("-key and -map are required")
	}
	seedB64, err := os.ReadFile(*keyPath)
	if err != nil {
		return err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(seedB64)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return errors.New("invalid private key")
	}
	priv := ed25519.NewKeyFromSeed(seed)

	raw, err := os.ReadFile(*mapPath)
	if err != nil {
		return err
	}
	var payload bytes.Buffer
	if err := json.Compact(&payload, raw); err != nil {
		return fmt.Errorf("the map is not valid JSON: %w", err)
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
		return fmt.Errorf("meta.status is %q: the gateway only accepts published maps", meta.Meta.Status)
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
	fmt.Printf("map version %d signed: %s\n", meta.Meta.Version, *out)
	return nil
}
