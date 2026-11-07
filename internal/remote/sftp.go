package remote

import (
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
func (c *Conn) SFTP() (net.Conn, error) {
	st, err := c.Open()
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(proto.HostRequest{Kind: proto.KindSFTP})
	if _, err := st.Write(append(b, '\n')); err != nil {
		st.Close()
		return nil, err
	}
	var r proto.HostReply
	line, err := readReply(st)
	if err != nil {
		st.Close()
		return nil, err
	}
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
	return st, nil
}

// readReply reads a line, a byte at a time: an answer is short, and the
// stream's rest stays unread, for SFTP's own.
func readReply(r io.Reader) ([]byte, error) {
	var line []byte
	var one [1]byte
	for len(line) < 64<<10 {
		n, err := r.Read(one[:])
		if n == 1 {
			if one[0] == '\n' {
				return line, nil
			}
			line = append(line, one[0])
		}
		if err != nil {
			return nil, err
		}
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
