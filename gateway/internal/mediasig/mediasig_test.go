package mediasig

import "testing"

func TestMatch(t *testing.T) {
	good := [][]byte{
		[]byte("\x00\x00\x00\x18ftypmp42"),
		{0x1A, 0x45, 0xDF, 0xA3, 0x01},
		[]byte("OggS\x00\x02"),
		[]byte("ID3\x04\x00"),
		[]byte("RIFF\x24\x00\x00\x00WAVE"),
		{0xFF, 0xFB, 0x90},
	}
	for _, h := range good {
		if !Match(h) {
			t.Errorf("Match(%q) = false", h)
		}
	}
	for _, h := range [][]byte{[]byte("<html><body>"), []byte("<?php echo"), nil, []byte("RIFF\x24\x00\x00\x00WEBP")} {
		if Match(h) {
			t.Errorf("Match(%q) = true", h)
		}
	}
}

func TestFileAndType(t *testing.T) {
	if !File("a/b/Video.MP4") || File("a/b.php") || File("mp4") {
		t.Error("File")
	}
	if !Type("video/mp4") || !Type("audio/mpeg") || Type("image/png") {
		t.Error("Type")
	}
}
