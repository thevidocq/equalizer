package sourceschema

import (
	"os"
	"testing"
)

func TestExample(t *testing.T) {
	doc, err := os.ReadFile("../../../schema/source/examples/wordpress.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(doc); err != nil {
		t.Fatal(err)
	}
	if Validate([]byte(`{"schema":"source-summary/1.0"}`)) == nil {
		t.Fatal("an incomplete summary passed")
	}
	if err := Validate([]byte(`{"schema":"source-summary/1.0","read_at":"2026-11-24T16:00:00Z","stack":"php","files":{"read":1,"skipped":0},"routes":[],"inputs":[],"risks":[],"components":[],"code":"<?php"}`)); err == nil {
		t.Fatal("a summary carrying code passed")
	}
}

func TestCopy(t *testing.T) {
	orig, _ := os.ReadFile("../../../schema/source/summary.schema.json")
	if string(orig) != string(schemaJSON) {
		t.Fatal("copy schema/source/summary.schema.json here again")
	}
}
