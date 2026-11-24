// Package keys keeps the reader's own ssh key: made once, in its state
// folder, the private half never leaving it. The panel shows the public
// half, to authorise on the web server for a read-only user.
package keys

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Ensure makes the key when missing and returns the private key's path and
// the public key.
func Ensure(state string) (string, string, error) {
	if err := os.MkdirAll(state, 0o700); err != nil {
		return "", "", err
	}
	priv := filepath.Join(state, "id_ed25519")
	if _, err := os.Stat(priv); os.IsNotExist(err) {
		out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "equalizer-source-reader", "-f", priv).CombinedOutput()
		if err != nil {
			return "", "", fmt.Errorf("ssh-keygen: %v: %s", err, strings.TrimSpace(string(out)))
		}
	}
	pub, err := os.ReadFile(priv + ".pub")
	if err != nil {
		return "", "", err
	}
	return priv, strings.TrimSpace(string(pub)), nil
}
