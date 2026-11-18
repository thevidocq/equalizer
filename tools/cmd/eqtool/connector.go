package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var connectorIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,62}$`)

type connectorEntry struct {
	ID          string `json:"id"`
	Tenant      string `json:"tenant"`
	App         string `json:"app"`
	TokenSHA256 string `json:"token_sha256"`
}

type connectorsFile struct {
	Connectors []connectorEntry `json:"connectors"`
}

// randomToken returns 32 random bytes in base64url.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func secret(args []string) error {
	fs := flag.NewFlagSet("secret", flag.ContinueOnError)
	out := fs.String("out", "relay.token", "token file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("%s already exists: not overwriting it", *out)
	}
	tok, err := randomToken()
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, []byte(tok+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("token: %s\n", *out)
	return nil
}

func connector(args []string) error {
	fs := flag.NewFlagSet("connector", flag.ContinueOnError)
	file := fs.String("file", "connectors.json", "relay connectors file")
	id := fs.String("id", "", "connector id (lower case letters, digits, - and _)")
	tenant := fs.String("tenant", "", "tenant")
	app := fs.String("app", "", "protected application")
	rotate := fs.Bool("rotate", false, "replace the token of an existing connector")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !connectorIDRe.MatchString(*id) {
		return fmt.Errorf("invalid -id %q", *id)
	}

	var doc connectorsFile
	raw, err := os.ReadFile(*file)
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("%s is not valid JSON: %w", *file, err)
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return err
	}

	tok, err := randomToken()
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(tok))
	hash := hex.EncodeToString(sum[:])
	found := false
	for i := range doc.Connectors {
		if doc.Connectors[i].ID != *id {
			continue
		}
		if !*rotate {
			return fmt.Errorf("connector %q already exists: use -rotate to replace its token", *id)
		}
		doc.Connectors[i].TokenSHA256 = hash
		found = true
	}
	if !found {
		if *rotate {
			return fmt.Errorf("connector %q does not exist", *id)
		}
		if *tenant == "" || *app == "" {
			return errors.New("-tenant and -app are required for a new connector")
		}
		doc.Connectors = append(doc.Connectors, connectorEntry{ID: *id, Tenant: *tenant, App: *app, TokenSHA256: hash})
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	// Write then rename: the relay never reads a half-written file.
	tmp := filepath.Join(filepath.Dir(*file), "."+filepath.Base(*file)+".tmp")
	if err := os.WriteFile(tmp, append(out, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, *file); err != nil {
		return err
	}
	fmt.Printf("connector %s registered in %s\n", *id, *file)
	fmt.Printf("EQ_CONNECTOR_TOKEN=%s\n", tok)
	fmt.Println("The token is shown only now: copy it to the connector's configuration.")
	return nil
}
