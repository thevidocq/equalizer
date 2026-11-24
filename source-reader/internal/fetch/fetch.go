package fetch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Spec is how to reach the site's files (the panel's SourceJob).
type Spec struct {
	Method   string // sftp, smb, local
	Host     string
	Port     int
	Username string
	Path     string
	Share    string
	Password string // smb only
}

// Env is what the reader brings: its key and folders.
type Env struct {
	Key   string // private key, for sftp
	State string // known_hosts lives here
	Work  string // fetched files go here, deleted after the read
	Site  string // the mounted folder, for local
}

// What the reader passes to rsync, ssh and smbclient: allowlists, the same
// as the panel's, so that nothing reaches a shell or a command string (the
// remote path goes through the server's shell, smbclient splits -c on ;).
var (
	safe      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._@-]{0,252}$`)
	safePath  = regexp.MustCompile(`^/[A-Za-z0-9._@+~ /-]{0,299}$`)
	safeShare = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._$-]{0,79}$`)
)

// Fetch returns the folder to read and a function that cleans up after.
func Fetch(ctx context.Context, s Spec, e Env) (string, func(), error) {
	nothing := func() {}
	switch s.Method {
	case "local":
		dir := filepath.Join(e.Site, filepath.Clean("/"+s.Path))
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return "", nothing, fmt.Errorf("the folder %s is not mounted in the reader", s.Path)
		}
		return dir, nothing, nil
	case "sftp", "smb":
		if !safe.MatchString(s.Host) || s.Username != "" && !safe.MatchString(s.Username) {
			return "", nothing, errors.New("host or username with characters a host or user name does not have")
		}
		if !safePath.MatchString(s.Path) || strings.Contains(s.Path+"/", "/../") || s.Method == "smb" && !safeShare.MatchString(s.Share) ||
			strings.ContainsAny(s.Password, "\x00\n\r") {
			return "", nothing, errors.New("path or share with characters the reader does not pass on")
		}
		if s.Method == "sftp" && e.Key == "" {
			return "", nothing, errors.New("the reader has no ssh key: is its state folder (EQ_SOURCE_STATE) writable?")
		}
		if err := os.MkdirAll(e.Work, 0o700); err != nil {
			return "", nothing, err
		}
		dir, err := os.MkdirTemp(e.Work, "read-")
		if err != nil {
			return "", nothing, err
		}
		clean := func() { _ = os.RemoveAll(dir) }
		ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		defer cancel()
		if s.Method == "sftp" {
			err = run(ctx, "rsync", RsyncArgs(s, e, dir), nil)
		} else {
			err = smb(ctx, s, e, dir)
		}
		if err != nil {
			clean()
			return "", nothing, err
		}
		if s.Method == "smb" {
			return filepath.Join(dir, "files"), clean, nil
		}
		return dir, clean, nil
	}
	return "", nothing, fmt.Errorf("unknown method %q", s.Method)
}

// RsyncArgs pulls only code files, read only, over ssh with the reader's
// key; the server's key is learnt the first time and checked after.
func RsyncArgs(s Spec, e Env, dir string) []string {
	port := s.Port
	if port == 0 {
		port = 22
	}
	ssh := fmt.Sprintf("ssh -i %s -p %d -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=%s -o ConnectTimeout=15",
		e.Key, port, filepath.Join(e.State, "known_hosts"))
	// --protect-args: the remote path reaches the server as one word, never
	// through its shell
	args := []string{"-r", "--protect-args", "--prune-empty-dirs", "--max-size=" + strconv.Itoa(MaxFileSize), "--timeout=120", "-e", ssh}
	for _, d := range skipDirs {
		args = append(args, "--exclude="+d+"/")
	}
	for _, n := range secretNames {
		args = append(args, "--exclude="+n)
	}
	args = append(args, "--include=*/")
	for ext := range codeExt {
		args = append(args, "--include=*"+ext)
	}
	for n := range codeNames {
		args = append(args, "--include="+n)
	}
	args = append(args, "--exclude=*")
	src := strings.TrimRight(s.Path, "/") + "/"
	if s.Username != "" {
		return append(args, s.Username+"@"+s.Host+":"+src, dir+"/")
	}
	return append(args, s.Host+":"+src, dir+"/")
}

// smb copies the code files of the share's folder, one mask at a time,
// then deletes at once what Wanted refuses (smbclient cannot skip a name).
// The password goes in a file only the reader reads, never on the command
// line.
func smb(ctx context.Context, s Spec, e Env, dir string) error {
	auth := filepath.Join(dir, ".auth")
	content := "username = " + s.Username + "\npassword = " + s.Password + "\n"
	if err := os.WriteFile(auth, []byte(content), 0o600); err != nil {
		return err
	}
	defer os.Remove(auth)
	files := filepath.Join(dir, "files")
	if err := os.MkdirAll(files, 0o700); err != nil {
		return err
	}
	masks := []string{"*.php", "*.phtml", "*.inc", "style.css", "composer.lock"}
	for _, m := range masks {
		cmd := fmt.Sprintf(`prompt OFF; recurse ON; mask "%s"; lcd "%s"; cd "%s"; mget *`, m, files, s.Path)
		args := []string{"//" + s.Host + "/" + s.Share, "-A", auth, "-c", cmd}
		if s.Port != 0 {
			args = append(args, "-p", strconv.Itoa(s.Port))
		}
		if err := run(ctx, "smbclient", args, []string{s.Password}); err != nil {
			return err
		}
	}
	return Prune(files)
}

// Prune deletes what the reader must not read from a fetched folder.
func Prune(root string) error {
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if info.IsDir() {
			if rel != "." && SkipDir(info.Name()) {
				return os.RemoveAll(p)
			}
			return nil
		}
		if !Wanted(filepath.ToSlash(rel), info.Size()) {
			return os.Remove(p)
		}
		return nil
	})
}

// run runs a tool and returns its last words on failure, without secrets.
func run(ctx context.Context, name string, args []string, secrets []string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	for _, s := range secrets {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, "***")
		}
	}
	if len(msg) > 400 {
		msg = msg[len(msg)-400:]
	}
	return fmt.Errorf("%s failed: %v: %s", name, err, msg)
}
