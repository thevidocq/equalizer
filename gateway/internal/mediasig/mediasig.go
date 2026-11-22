// Package mediasig recognises audio and video files by their signature.
//
// Media assets are streamed, not buffered, so only the first bytes are
// checked: enough to know that a file served as video/mp4 is a video and not
// a page or a script.
package mediasig

import (
	"bytes"
	"path"
	"strings"
)

// HeadSize is how many bytes Match needs.
const HeadSize = 12

// extensions of the files served as streamed media.
var extensions = map[string]bool{
	".mp4": true, ".m4v": true, ".mov": true, ".webm": true, ".ogv": true,
	".mp3": true, ".m4a": true, ".aac": true, ".oga": true, ".ogg": true,
	".opus": true, ".wav": true, ".flac": true,
}

// File reports whether a file name looks like audio or video. The name only
// chooses how the file is read; type and signature decide whether it is served.
func File(name string) bool { return extensions[strings.ToLower(path.Ext(name))] }

// Type reports whether a media type is audio or video.
func Type(mediaType string) bool {
	return strings.HasPrefix(mediaType, "video/") || strings.HasPrefix(mediaType, "audio/")
}

// Match reports whether head, the first bytes of a file, is the start of an
// audio or video container.
func Match(head []byte) bool {
	switch {
	case len(head) >= 8 && string(head[4:8]) == "ftyp": // MP4, MOV, M4A, 3GP
		return true
	case bytes.HasPrefix(head, []byte{0x1A, 0x45, 0xDF, 0xA3}): // WebM, Matroska
		return true
	case bytes.HasPrefix(head, []byte("OggS")), bytes.HasPrefix(head, []byte("fLaC")),
		bytes.HasPrefix(head, []byte("ID3")): // Ogg, FLAC, MP3 with tags
		return true
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return true
	case len(head) >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0: // MPEG audio / ADTS frame
		return true
	}
	return false
}
