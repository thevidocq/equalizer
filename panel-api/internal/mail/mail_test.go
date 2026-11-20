package mail

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
)

// A sender with a display name: the envelope carries the bare address
// (servers answer 501 to "MAIL FROM:<Name <addr>>"), the header the name.
func TestSMTPDisplayName(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan []string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		var lines []string
		say := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		say("220 test")
		data := false
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				break
			}
			l = strings.TrimRight(l, "\r\n")
			lines = append(lines, l)
			switch {
			case data && l == ".":
				data = false
				say("250 ok")
			case data:
			case strings.HasPrefix(l, "EHLO"), strings.HasPrefix(l, "HELO"):
				say("250 test")
			case strings.HasPrefix(l, "MAIL FROM:<noreply@example.com>"), strings.HasPrefix(l, "RCPT TO:"):
				say("250 ok")
			case strings.HasPrefix(l, "MAIL"):
				say("501 Syntax error in parameters or arguments")
			case l == "DATA":
				data = true
				say("354 go")
			case l == "QUIT":
				say("221 bye")
				got <- lines
				return
			default:
				say("250 ok")
			}
		}
		got <- lines
	}()
	s := SMTP{Addr: ln.Addr().String(), From: "Equalizer <noreply@example.com>"}
	if err := s.Send(context.Background(), Message{To: "boss@acme.test", Subject: "Hi", Body: "hello"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	lines := strings.Join(<-got, "\n")
	if !strings.Contains(lines, "MAIL FROM:<noreply@example.com>") || !strings.Contains(lines, `From: "Equalizer" <noreply@example.com>`) {
		t.Fatalf("dialogue:\n%s", lines)
	}
}

func TestProblem(t *testing.T) {
	if Problem(ErrDisabled) != ProblemNotConfigured || Problem(nil) != "" {
		t.Fatal("disabled")
	}
	err := SMTP{Addr: "127.0.0.1:1", From: "a@example.com"}.Send(context.Background(), Message{To: "b@example.com"})
	if Problem(err) != ProblemUnreachable {
		t.Fatalf("closed port: %v", err)
	}
	if Problem(SMTP{Addr: "127.0.0.1:1", From: "Not an address"}.Send(context.Background(), Message{To: "b@example.com"})) != ProblemSenderRejected {
		t.Fatal("bad sender")
	}
}
