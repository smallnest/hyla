package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/xingkaixin/mygo-native-ui/ui/qrcode"

	"hyla/internal/pocket"
	"hyla/internal/remote"
	"hyla/internal/settings"
)

// pocketCmd is `hyla pocket`: this machine as a host for
// HylaPocket on a phone. The phone reaches it over tailcat, on this
// machine's tailcat listener, and the code it scans is that listener's
// pair code: one secret, which is the address and the admission both.
func pocketCmd(args []string) error {
	sub := "code"
	if len(args) > 0 && args[0] != "" {
		sub = args[0]
	}
	switch sub {
	case "enable":
		return tailcatCmd([]string{"enable"})
	case "disable":
		return tailcatCmd([]string{"disable"})
	case "code":
		return pocketCode(false)
	case "qr":
		return pocketCode(true)
	case "tcp":
		return pocketTCP(args[1:])
	case "addr":
		return pocketAddr()
	case "token":
		return pocketToken()
	}
	return usageErr("hyla pocket enable|disable|code|qr|tcp|addr|token")
}

// pocketTCP turns this machine's own address on and off: a tcp://
// listener on the network, which Hyla and HylaPocket reach by the address
// and the password (hyla pocket addr).
func pocketTCP(args []string) error {
	sub := "enable"
	if len(args) > 0 {
		sub = args[0]
	}
	set := settings.Load()
	switch sub {
	case "enable":
		addr := ""
		if len(args) > 1 {
			addr = strings.TrimSpace(args[1])
		}
		spec, err := remote.ParseListen("tcp://" + addr)
		if err != nil {
			return usageErr("%v", err)
		}
		if set.ListenToken == "" {
			token, err := remote.RandomToken()
			if err != nil {
				return err
			}
			set.ListenToken = token
		}
		set.Listen = slices.DeleteFunc(set.Listen, func(l string) bool {
			s, err := remote.ParseListen(l)
			return err == nil && s.Kind == "tcp"
		})
		set.Listen = append(set.Listen, spec.String())
		if err := settings.Save(set); err != nil {
			return err
		}
		fmt.Printf("hylad listens on %s: hyla pocket addr shows where, and the password\n", spec)
		return nil
	case "disable":
		set.Listen = slices.DeleteFunc(set.Listen, func(l string) bool {
			s, err := remote.ParseListen(l)
			return err == nil && s.Kind == "tcp"
		})
		return settings.Save(set)
	}
	return usageErr("hyla pocket tcp enable [address:port]|disable")
}

// pocketAddr shows where this machine's tcp listener is, and the password
// that admits a client: HylaPocket takes them as an address and a token.
func pocketAddr() error {
	set := settings.Load()
	i := slices.IndexFunc(set.Listen, func(l string) bool {
		s, err := remote.ParseListen(l)
		return err == nil && s.Kind == "tcp"
	})
	if i < 0 {
		return errors.New("hylad listens on no address of this machine: hyla pocket tcp enable")
	}
	spec, _ := remote.ParseListen(set.Listen[i])
	port := spec.Port
	if spec.Host != "" && !net.ParseIP(spec.Host).IsUnspecified() {
		fmt.Printf("%s\n", net.JoinHostPort(spec.Host, port))
	} else {
		for _, a := range remote.LocalAddrs() {
			fmt.Printf("%s\n", net.JoinHostPort(a, port))
		}
	}
	if set.ListenToken == "" {
		return errors.New("no password: hyla pocket tcp enable makes one")
	}
	fmt.Fprintf(os.Stderr, "the password, which the token field takes: %s\n", set.ListenToken)
	return nil
}

// pocketToken prints the password of this machine's listeners alone, for
// a client that asks for one.
func pocketToken() error {
	set := settings.Load()
	if set.ListenToken == "" {
		return errors.New("no password yet: hyla pocket tcp enable makes one")
	}
	fmt.Println(set.ListenToken)
	return nil
}

// pocketCode shows this machine's code: as the line HylaPocket takes
// pasted in, or with --qr as the QR code, drawn in the terminal.
func pocketCode(draw bool) error {
	s := settings.Load()
	if !s.Tailcat {
		return errors.New("tailcat is off, so HylaPocket cannot reach this machine: hyla pocket enable")
	}
	if _, _, ok := s.TailcatListen(); !ok {
		return errors.New("hylad has no tailcat listener for HylaPocket to reach: hyla pocket enable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	addr, err := remote.LocalTailcatAddress(ctx)
	if err != nil {
		return err
	}
	code := pocket.Code{Pair: remote.TailcatCode(addr), Name: remote.LocalName()}
	text := code.CodeURL()
	if draw {
		q, err := qrcode.Encode(text, qrcode.L)
		if err != nil {
			return err
		}
		// Two modules a character, as dark on light: a terminal's camera
		// scan needs the quiet zone to be white too.
		n := q.Size + 8
		for y := 0; y < n; y++ {
			var b strings.Builder
			for x := 0; x < n; x++ {
				dark := x >= 4 && y >= 4 && x < q.Size+4 && y < q.Size+4 && q.Dark(x-4, y-4)
				if dark {
					b.WriteString("██")
				} else {
					b.WriteString("  ")
				}
			}
			fmt.Println(b.String())
		}
	}
	fmt.Println(text)
	fmt.Fprintln(os.Stderr, "the pair code in it reaches every session of this machine, to anyone who has it: hyla tailcat rotate makes another, and hyla pocket disable stops HylaPocket")
	return nil
}
