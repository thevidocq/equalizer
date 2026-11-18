package upload

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// Clamd scans with a ClamAV daemon (INSTREAM over TCP).
type Clamd struct {
	Addr    string // host:port, e.g. clamav:3310
	Timeout time.Duration
}

// Scan implements Scanner.
func (c *Clamd) Scan(ctx context.Context, data []byte) error {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return fmt.Errorf("clamd unreachable: %w", err)
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return err
	}
	var size [4]byte
	for off := 0; off < len(data); off += 64 << 10 {
		chunk := data[off:min(off+64<<10, len(data))]
		binary.BigEndian.PutUint32(size[:], uint32(len(chunk)))
		if _, err := conn.Write(size[:]); err != nil {
			return err
		}
		if _, err := conn.Write(chunk); err != nil {
			return err
		}
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return err
	}
	reply, err := bufio.NewReader(conn).ReadBytes(0)
	if err != nil && len(reply) == 0 {
		return fmt.Errorf("clamd answered nothing: %w", err)
	}
	reply = bytes.TrimRight(reply, "\x00\n")
	switch {
	case bytes.HasSuffix(reply, []byte("OK")):
		return nil
	case bytes.HasSuffix(reply, []byte("FOUND")):
		return ErrInfected
	}
	return fmt.Errorf("clamd: %q", reply)
}
