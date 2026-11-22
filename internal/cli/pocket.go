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

	"hyla/internal/pocket"
	"hyla/internal/remote"
	"hyla/internal/settings"
)

// pocketCmd is `hyla pocket`: this machine as a host for
// HylaPocket on a phone. The phone reaches it over tailcat, on this
// machine's tailcat listener, or over BeaconTower, on its beacontower://
// one, and the code it scans is that listener's pair code: one secret,
// which is the address and the admission both.
//
// enable and disable are tailcat's alone, as they were before
// BeaconTower: they stay the tailcat switch, and BeaconTower has its own
// (hyla beacontower enable).
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
	case "code", "qr":
		return pocketCode(sub == "qr")
	case "beacon":
		return pocketBeacon(false)
	case "tcp":
		return pocketTCP(args[1:])
	case "addr":
		return pocketAddr()
	case "token":
		return pocketToken()
	}
	return usageErr("hyla pocket enable|disable|code|qr|beacon|tcp|addr|token")
}

// pocketBeacon prints the code HylaPocket scans to reach this machine over
// BeaconTower: the beacontower:// pair code of its key, as hyla pocket qr
// shows the tailcat one, so that a phone reads either the same way.
//
// Why the key is read, not made: a code printed here must name the machine
// hylad will be, and a key made now would not be the one hylad has.
func pocketBeacon(draw bool) error {
	set := settings.Load()
	if set.BeaconTowerRelay == "" {
		return errors.New("beacontower-relay is not configured, so no BeaconTower code reaches this machine: hyla beacontower relay <relay-url>")
	}
	if !remote.ValidBeaconRelay(set.BeaconTowerRelay) {
		return fmt.Errorf("beacontower-relay %q is not a BeaconTower relay: hyla beacontower relay <relay-url>", set.BeaconTowerRelay)
	}
	k, err := beaconKey()
	if err != nil {
		return err
	}
	code, err := remote.BeaconTowerAddress(k, set.BeaconTowerRelay, set.BeaconTowerToken)
	if err != nil {
		return err
	}
	// The code field holds the bare code: CodeURL also strips a scheme, but
	// the code line is cleaner without one.
	text := pocket.Code{Pair: code, Name: remote.LocalName()}.CodeURL()
	if draw {
		printTerminalQR(text)
	}
	if _, on := set.BeaconTowerListen(); !on {
		fmt.Fprintln(os.Stderr, "hylad does not listen over beacontower here, so no phone reaches this code: hyla beacontower enable")
	}
	fmt.Println(text)
	fmt.Fprintln(os.Stderr, "the pair code in it reaches every session of this machine, to anyone who has it: hyla beacontower rotate makes another")
	return nil
}

// beaconKey is this machine's BeaconTower key, which the code is made
// of; none is made here, since hylad makes its key when its listener
// first starts, and the code must be that one.
func beaconKey() (*remote.BeaconTowerKey, error) {
	path := remote.BeaconTowerKeyPath()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("this machine has no BeaconTower key yet: hyla beacontower enable starts hylad's BeaconTower listener, which makes one")
	}
	return remote.LoadBeaconTowerKey(path)
}

// pocketCode shows this machine's code: as the line HylaPocket takes
// pasted in, or with --qr as the QR code, drawn in the terminal.
//
// A machine that listens over both tailcat and BeaconTower prints both
// codes, each on a line of its own, with a label line before it on stderr: a
// phone scans one code per host, and the label says which way that code
// reaches the machine. The code is on stdout as is, so that a script takes
// the same lines as before. A machine BeaconTower alone reaches, with
// beacontower-relay set, prints its code on the plain line alone, as hyla
// pocket beacon does.
func pocketCode(draw bool) error {
	s := settings.Load()
	_, _, tailcat := s.TailcatListen()
	_, beacon := s.BeaconTowerListen()
	if tailcat && beacon {
		return pocketBothCodes(draw)
	}
	// A machine whose only listener is BeaconTower's keeps the plain line,
	// for a user, and a script, that paste one line: no label to strip.
	if beacon {
		return pocketBeacon(draw)
	}
	return pocketTailcatCode(draw)
}

// pocketBothCodes prints the code of each of this machine's two listeners,
// labelled: a phone scans one code per host, and a user picks the line of
// the one the phone reaches.
func pocketBothCodes(draw bool) error {
	errs := 0
	fmt.Fprintln(os.Stderr, "the tailcat code:")
	if err := pocketTailcatCode(draw); err != nil {
		errs++
		fmt.Fprintf(os.Stderr, "tailcat: no code, %v\n", err)
	}
	fmt.Fprintln(os.Stderr, "the BeaconTower code:")
	if err := pocketBeacon(draw); err != nil {
		errs++
		fmt.Fprintf(os.Stderr, "beacontower: no code, %v\n", err)
	}
	if errs == 2 {
		return errors.New("no code reaches this machine")
	}
	return nil
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
// pasted in, or as the QR code, drawn in the terminal.
func pocketTailcatCode(draw bool) error {
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
		printTerminalQR(text)
	}
	fmt.Println(text)
	fmt.Fprintln(os.Stderr, "the pair code in it reaches every session of this machine, to anyone who has it: hyla tailcat rotate makes another, and hyla pocket disable stops HylaPocket")
	return nil
}
