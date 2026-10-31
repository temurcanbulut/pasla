package node

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func incomingForTest(payload string) *Transfer {
	return &Transfer{Files: []FileMeta{{Name: "invoice.txt", Size: int64(len(payload))}}, ctx: context.Background()}
}

func TestReceivePreservesExistingPartialName(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "hash_failure"}[broken], func(t *testing.T) {
			dir := t.TempDir()
			old := filepath.Join(dir, "invoice.txt.paslapart")
			if err := os.WriteFile(old, []byte("keep existing data"), 0o600); err != nil {
				t.Fatal(err)
			}
			dest := planDestinations(dir, [][]string{{"invoice.txt"}})[0]
			var wantHash func() string
			if broken {
				wantHash = func() string { return "wrong" }
			}
			n := &Node{}
			transfer := incomingForTest("new")
			err := n.receiveFile(transfer, 0, dest, strings.NewReader("new"), wantHash)
			if (err != nil) != broken {
				t.Fatalf("unexpected receive error: %v", err)
			}
			data, err := os.ReadFile(old)
			if err != nil || string(data) != "keep existing data" {
				t.Fatalf("existing partial changed: %q, %v", data, err)
			}
			if broken {
				if _, err := os.Lstat(dest); !os.IsNotExist(err) {
					t.Fatalf("failed receive left destination: %v", err)
				}
				if transfer.done.Load() != 0 {
					t.Fatal("failed receive retained progress")
				}
			} else {
				if data, err := os.ReadFile(dest); err != nil || string(data) != "new" {
					t.Fatalf("bad received file: %q, %v", data, err)
				}
			}
			partials, err := filepath.Glob(filepath.Join(dir, ".pasla-*.part"))
			if err != nil || len(partials) != 0 {
				t.Fatalf("temporary file leak: %v, %v", partials, err)
			}
		})
	}
}

func TestReceiveRefusesDestinationCreatedAfterPlanning(t *testing.T) {
	dir := t.TempDir()
	dest := planDestinations(dir, [][]string{{"invoice.txt"}})[0]
	if err := os.WriteFile(dest, []byte("keep final data"), 0o600); err != nil {
		t.Fatal(err)
	}
	n := &Node{}
	if err := n.receiveFile(incomingForTest("new"), 0, dest, strings.NewReader("new"), nil); err == nil {
		t.Fatal("overwrote existing destination")
	}
	if data, err := os.ReadFile(dest); err != nil || string(data) != "keep final data" {
		t.Fatalf("destination changed: %q, %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file leak: %v, %v", entries, err)
	}
}

func TestConcurrentReceiversDoNotOverwriteEachOther(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "invoice.txt")
	n := &Node{}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, payload := range []string{"first payload", "second payload"} {
		go func(value string) {
			<-start
			errs <- n.receiveFile(incomingForTest(value), 0, dest, strings.NewReader(value), nil)
		}(payload)
	}
	close(start)
	succeeded := 0
	for range 2 {
		if <-errs == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("expected one completed file, got %d", succeeded)
	}
	data, err := os.ReadFile(dest)
	if err != nil || (string(data) != "first payload" && string(data) != "second payload") {
		t.Fatalf("corrupt destination: %q, %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file leak: %v, %v", entries, err)
	}
}
