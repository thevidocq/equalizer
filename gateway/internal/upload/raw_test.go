package upload

import (
	"bytes"
	"testing"
)

func TestCheckRaw(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 40)...)
	pdf := []byte("%PDF-1.7\n1 0 obj")
	zip := append([]byte("PK\x03\x04"), bytes.Repeat([]byte{1}, 40)...)
	for _, c := range []struct {
		declared string
		head     []byte
		want     string
	}{
		{"image/png", png, "image/png"},
		{"image/jpeg", png, "image/png"}, // a picture, sent with its real type
		{"application/octet-stream", png, "application/octet-stream"},
		{"application/pdf", pdf, "application/pdf"},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", zip, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"video/mp4", []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00"), "video/mp4"},
		{"application/octet-stream", bytes.Repeat([]byte{7, 9}, 30), "application/octet-stream"},
		{"image/png", []byte("<html><script>alert(1)</script>"), ""},
		{"image/png", pdf, ""},
		{"application/pdf", png, ""},
		{"application/octet-stream", []byte("MZ\x90\x00 This program"), ""},
		{"application/octet-stream", []byte("#!/bin/sh\nid"), ""},
		{"application/octet-stream", []byte("<?php system($_GET[c]);"), ""},
		{"image/gif", bytes.Repeat([]byte{7, 9}, 30), ""},
	} {
		got, err := CheckRaw(c.declared, c.head)
		if c.want == "" && err == nil || c.want != "" && got != c.want {
			t.Errorf("%s %q: %q %v", c.declared, c.head[:min(len(c.head), 12)], got, err)
		}
	}
	if IsRawType("image/svg+xml") || !IsRawType("image/webp") || IsRawType("text/html") {
		t.Error("raw types")
	}
}
