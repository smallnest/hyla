package pocket

import (
	"fmt"
	"net/url"
	"strings"
)

// Code is this machine as the QR code on it carries it: the pair code of
// its tailcat key, which is both where HylaPocket reaches this machine and
// the secret that admits it, and the name the phone saves it under. There
// is no token: a connection that came over tailcat reached the pair code,
// and that is all that admits it.
type Code struct {
	Pair string
	Name string
}

// CodeURL is the code as HylaPocket reads it, and as hyla prints it:
// hyla://pair?code=tc…&name=….
func (c Code) CodeURL() string {
	q := url.Values{}
	q.Set("code", strings.TrimPrefix(strings.TrimSpace(c.Pair), "tailcat://"))
	if name := strings.TrimSpace(c.Name); name != "" {
		q.Set("name", name)
	}
	return "hyla://pair?" + q.Encode()
}

// ParseCodeURL reads a code URL, which HylaPocket's scanner sends back as
// the text it read.
func ParseCodeURL(s string) (Code, error) {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || !strings.EqualFold(u.Scheme, "hyla") || u.Host != "pair" {
		return Code{}, fmt.Errorf("%q is not a host code", s)
	}
	q := u.Query()
	c := Code{Pair: strings.TrimPrefix(q.Get("code"), "tailcat://"), Name: q.Get("name")}
	if !ValidPairCode(c.Pair) {
		return Code{}, fmt.Errorf("%q is not a host code", s)
	}
	return c, nil
}
