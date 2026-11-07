package remote

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
)

// TestSFTP checks that a host serves SFTP over a stream of a remote
// connection, as scp puts files on it: files and folders go in, and come
// back as they were.
func TestSFTP(t *testing.T) {
	s := echoServer(t)
	s.SFTP = ServeSFTP
	c, done := dialPipe(t, s, "")
	defer func() {
		c.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the server did not end")
		}
	}()
	conn, err := c.SFTP()
	if err != nil {
		t.Fatal(err)
	}
	cl, err := sftp.NewClientPipe(conn, conn)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	dir := t.TempDir()
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.MkdirAll(dir + "/sub"); err != nil {
		t.Fatal(err)
	}
	want := []byte("dropped\n")
	w, err := cl.Create(dir + "/sub/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(want); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dir + "/sub/a.txt")
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("read %q, %v, want %q", got, err, want)
	}
	names, err := cl.ReadDir(dir + "/sub")
	if err != nil || len(names) != 1 || names[0].Name() != "a.txt" {
		t.Errorf("ReadDir = %v, %v", names, err)
	}
}
