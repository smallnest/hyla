package remote

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/pkg/sftp"

	"hyla/internal/proto"
)

// SFTP opens a stream in which the host serves SFTP, as scp puts files on
// it and gets them, from the user's namespaces where hylad runs: the
// client closes it with the client it makes of it.
func (c *Conn) SFTP() (net.Conn, error) { return OpenSFTP(c) }

// Opener opens streams to a host's hylad, as a remote connection does
// (Conn.Open).
type Opener interface {
	Open() (net.Conn, error)
}

// OpenSFTP opens a stream in which a host serves SFTP (SFTP). The
// stream's reads go through a reader of the answer: a connection may read
// ahead, as a yamux stream's does, and the SFTP client must read what it
// read.
func OpenSFTP(c Opener) (net.Conn, error) {
	st, err := c.Open()
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(proto.HostRequest{Kind: proto.KindSFTP})
	if _, err := st.Write(append(b, '\n')); err != nil {
		st.Close()
		return nil, err
	}
	// The request waits for the host's answer, which a buffered connection
	// of a stream's window: the answer's reader starts now.
	br := bufio.NewReader(st)
	line, err := readReply(br)
	if err != nil {
		st.Close()
		return nil, err
	}
	var r proto.HostReply
	if err := json.Unmarshal(line, &r); err != nil {
		st.Close()
		return nil, fmt.Errorf("the host answered what is not Hyla: %q", line)
	}
	if !r.OK {
		st.Close()
		msg := r.Error
		if msg == "" {
			msg = "the host serves no SFTP"
		}
		return nil, errors.New(msg)
	}
	return &readAhead{br, st}, nil
}

// readAhead is a connection whose reads go through a reader that read
// ahead of it.
type readAhead struct {
	r *bufio.Reader
	net.Conn
}

func (c readAhead) Read(p []byte) (int, error) { return c.r.Read(p) }

// readReply reads a line, a byte at a time from the reader: an answer is
// short, and what the stream has beyond it waits in the reader, for
// SFTP's own.
func readReply(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for len(line) < 64<<10 {
		c, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if c == '\n' {
			return line, nil
		}
		line = append(line, c)
	}
	return nil, errors.New("an answer too long")
}

// ServeSFTP serves SFTP on a stream, over a bridge of its own, until it
// closes.
func ServeSFTP(conn net.Conn) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("sftp: %v", r)
		}
	}()
	srv, err := sftp.NewServer(conn)
	if err != nil {
		return err
	}
	defer srv.Close()
	err = srv.Serve()
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
