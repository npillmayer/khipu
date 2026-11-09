package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestViewPipeline(t *testing.T) {
	for _, breaker := range []string{"firstfit", "knuthplass"} {
		for _, direction := range []string{"ltr", "rtl"} {
			for _, source := range []string{"", "Hello world <&>", "aa אב גד bb", "a ab\u00adcd", "one\ntwo\n"} {
				t.Run(breaker+"/"+direction+"/"+source, func(t *testing.T) {
					var a, b bytes.Buffer
					for _, out := range []*bytes.Buffer{&a, &b} {
						if err := runView(t.Context(), "-", "-", breaker, direction, 25, strings.NewReader(source), out); err != nil {
							t.Fatal(err)
						}
					}
					if !bytes.Equal(a.Bytes(), b.Bytes()) {
						t.Fatal("nondeterministic output")
					}
					d := xml.NewDecoder(&a)
					for {
						_, err := d.Token()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Fatal(err)
						}
					}
					if !strings.Contains(b.String(), "Synthetic shaping") || !strings.Contains(b.String(), "source bytes") {
						t.Fatal("missing provenance/diagnostic labels")
					}
				})
			}
		}
	}
}

func TestViewErrorsPreserveOutput(t *testing.T) {
	output := filepath.Join(t.TempDir(), "result.svg")
	if err := os.WriteFile(output, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		breaker, direction, text string
		width                    int
	}{
		{"bad", "ltr", "hello", 50}, {"firstfit", "bad", "hello", 50},
		{"firstfit", "ltr", "hello", 0}, {"firstfit", "ltr", "hello", 32767},
		{"firstfit", "ltr", string([]byte{0xff}), 50},
		{"firstfit", "ltr", strings.Repeat("a", (1<<20)+1), 50},
	} {
		if err := runView(t.Context(), "-", output, tc.breaker, tc.direction, tc.width, strings.NewReader(tc.text), io.Discard); err == nil {
			t.Fatal("expected error")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := runView(ctx, "-", output, "firstfit", "ltr", 50, strings.NewReader("hello"), io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	got, err := os.ReadFile(output)
	if err != nil || string(got) != "keep" {
		t.Fatalf("output overwritten: %q %v", got, err)
	}
}

func TestViewFiles(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "input.txt"), filepath.Join(dir, "output.svg")
	if err := os.WriteFile(input, []byte("hello world"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runView(t.Context(), input, output, "firstfit", "ltr", 100, nil, nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(output)
	if err != nil || !bytes.Contains(b, []byte("</svg>")) {
		t.Fatalf("output: %s %v", b, err)
	}
}
