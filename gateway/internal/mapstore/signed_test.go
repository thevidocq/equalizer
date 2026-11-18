package mapstore

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const testMap = `{"meta":{"schema":"gateway-map/1.0","tenant":"t","app":"a","version":3,"status":"published"},
"legacy":{"base_url":"http://10.0.0.1:8080","reach":{"mode":"direct","allow":["10.0.0.1:8080"]}},
"roles":{"anonymous":"anonimo","list":{"anonimo":{}}},
"routes":[{"id":"home","path":"/","method":"GET","roles":["anonimo"],"render":"faithful","legacy_request":{"method":"GET","path":"/index.jsp"}}],
"forms":[{"id":"contatti","route":"/contatti","roles":["anonimo"]}]}`

func sign(t *testing.T, priv ed25519.PrivateKey, payload string, version int, keyID string) []byte {
	t.Helper()
	doc, err := json.Marshal(SignedMap{
		Version:   version,
		Payload:   base64.StdEncoding.EncodeToString([]byte(payload)),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(payload))),
		KeyID:     keyID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)

	m, _, err := Verify(sign(t, priv, testMap, 3, "k1"), pub, "k1")
	if err != nil {
		t.Fatalf("mappa valida rifiutata: %v", err)
	}
	if m.Meta.App != "a" || len(m.Routes) != 1 {
		t.Fatalf("mappa decodificata male: %+v", m.Meta)
	}

	cases := map[string]struct {
		doc  []byte
		want error
	}{
		"firma di un'altra chiave": {sign(t, otherPriv, testMap, 3, "k1"), ErrBadSignature},
		"id chiave diverso":        {sign(t, priv, testMap, 3, "k2"), ErrWrongKey},
		"versione diversa":         {sign(t, priv, testMap, 4, "k1"), ErrVersionMatch},
	}
	for name, c := range cases {
		if _, _, err := Verify(c.doc, pub, "k1"); !errors.Is(err, c.want) {
			t.Errorf("%s: atteso %v, ottenuto %v", name, c.want, err)
		}
	}

	// payload modificato dopo la firma
	var sm SignedMap
	_ = json.Unmarshal(sign(t, priv, testMap, 3, "k1"), &sm)
	sm.Payload = base64.StdEncoding.EncodeToString([]byte(testMap + " "))
	tampered, _ := json.Marshal(sm)
	if _, _, err := Verify(tampered, pub, "k1"); !errors.Is(err, ErrBadSignature) {
		t.Errorf("payload alterato: atteso ErrBadSignature, ottenuto %v", err)
	}
}

func TestStoreKeepsPreviousMapOnError(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	path := filepath.Join(t.TempDir(), "current.json")
	s := NewStore(path, pub, "")

	if _, err := s.Reload(); err == nil {
		t.Fatal("atteso errore senza file")
	}
	if s.Current() != nil {
		t.Fatal("nessuna mappa attesa")
	}

	_ = os.WriteFile(path, sign(t, priv, testMap, 3, "k1"), 0o600)
	if _, err := s.Reload(); err != nil {
		t.Fatalf("reload valido: %v", err)
	}
	_ = os.WriteFile(path, sign(t, otherPriv, testMap, 3, "k1"), 0o600)
	if _, err := s.Reload(); err == nil {
		t.Fatal("attesa firma non valida")
	}
	if a := s.Current(); a == nil || a.Model.Meta.Version != 3 {
		t.Fatal("la mappa precedente deve restare attiva")
	}
	if _, err := s.Current().Router.Match("POST", "/contatti"); err != nil {
		t.Fatalf("form non instradato: %v", err)
	}
}
