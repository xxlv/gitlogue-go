package main

import (
	"strings"
	"testing"
)

func TestListenRejectsInspect(t *testing.T) {
	t.Parallel()
	err := run([]string{"--listen", "--inspect"})
	if err == nil || !strings.Contains(err.Error(), "--listen") {
		t.Fatalf("got %v", err)
	}
}

func TestListenRejectsRange(t *testing.T) {
	t.Parallel()
	err := run([]string{"--listen", "main..feature"})
	if err == nil || !strings.Contains(err.Error(), "--wip") {
		t.Fatalf("got %v", err)
	}
}
