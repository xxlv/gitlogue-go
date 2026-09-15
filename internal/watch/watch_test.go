package watch

import "testing"

func TestSnapshotNilEngine(t *testing.T) {
	t.Parallel()
	d, sig, err := Snapshot(nil, "HEAD", "")
	if d != nil || sig != "" || err != nil {
		t.Fatalf("diff=%v sig=%q err=%v", d, sig, err)
	}
}

func TestListenReturnsCmd(t *testing.T) {
	t.Parallel()
	if Listen(nil, "HEAD", "", "") == nil {
		t.Fatal("Listen must return a Cmd")
	}
}
