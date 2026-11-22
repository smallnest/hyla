package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/xingkaixin/mygo-native-ui/ui/qrcode"

	"hyla/internal/build"
	"hyla/internal/client"
	"hyla/internal/daemon"
	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/service"
	"hyla/internal/settings"
)

// connect opens a control connection to this machine's hylad, started as
// needed.
func connect() (*client.Client, func(), error) {
	c, err := client.Connect(false)
	if err != nil {
		return nil, nil, err
	}
	return c, func() { c.Close() }, nil
}

// versionCmd prints the version of Hyla.
func versionCmd() error {
	v := build.Version()
	if v == "" {
		v = "dev"
	}
	if build.Server {
		v += ", server build"
	}
	fmt.Printf("hyla %s (protocol %d)\n", v, proto.Version)
	return nil
}

// tailcatCmd is `hyla tailcat`: this machine reachable over Tailcat, by a
// pair code rather than an SSH login (WireGuard between the two, a DERP
// relay only to find each other). HylaPocket rides the same listener, and
// `hyla pocket` prints its code as a QR code.
//
// `hyla p2p` is the other way: this machine on its network, by an address
// and a password (p2pCmd). p2p and tailcat are two ways to reach a
// machine, not one under two names, so each command says its own name.
func tailcatCmd(args []string) error {
	set := settings.Load()
	// The relay the configuration names is the one the code is made at,
	// as hylad's listener and the app make theirs.
	remote.SetRelay(set.TailcatRelay)
	spec, port, on := set.TailcatListen()
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status", "show":
		switch {
		case !set.Tailcat:
			fmt.Println("tailcat: off (tailcat = false in the configuration)")
			fmt.Println("  turn it on with tailcat = true, then hyla tailcat enable [port]")
		case !on:
			fmt.Println("tailcat: on, not listening")
			fmt.Println("  hyla tailcat enable [port]   listen over tailcat (default port 7473)")
			fmt.Println("  hyla p2p                     the machines reached by address and password")
			fmt.Println("  hyla pocket qr               the same code, as a QR code for a phone")
		default:
			fmt.Printf("tailcat: listening on %s (port %s)\n", spec, port)
			if code, err := localPairCode(); err == nil {
				fmt.Printf("  pair code: tailcat://%s\n", code)
			} else {
				fmt.Printf("  pair code: %v\n", err)
			}
			fmt.Fprintln(os.Stderr, "  the pair code is the secret: anyone with it can connect; hyla tailcat rotate makes a new one")
			fmt.Fprintln(os.Stderr, "  HylaPocket scans the same code: hyla pocket qr")
		}
		return nil
	case "code":
		if !set.Tailcat {
			return errors.New("tailcat is off: hyla tailcat enable")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		code, err := remote.LocalTailcatAddress(ctx)
		cancel()
		if err != nil {
			return err
		}
		fmt.Println("tailcat://" + code)
		fmt.Fprintln(os.Stderr, "the pair code is the secret: anyone with it can connect; hyla tailcat rotate makes a new one")
		if !on {
			fmt.Fprintln(os.Stderr, "hylad does not listen over tailcat here: hyla tailcat enable")
		} else {
			fmt.Fprintf(os.Stderr, "hylad listens on port %s (hyla tailcat port sets it)\n", port)
		}
		return nil
	case "enable":
		if !set.Tailcat {
			return errors.New("tailcat is off in the configuration (tailcat = false): hyla tailcat needs tailcat = true")
		}
		if !on {
			set.SetTailcatListen(settings.TailcatListenPortDefault())
		}
		if len(args) > 1 {
			if !settings.ValidListenPort(args[1]) {
				return usageErr("%q is not a port", args[1])
			}
			set.SetTailcatListen(args[1])
		}
		if err := settings.Save(set); err != nil {
			return err
		}
		fmt.Println("hylad listens over tailcat: hyla tailcat code")
		return nil
	case "disable":
		set.SetTailcatListen("")
		return settings.Save(set)
	case "port":
		if len(args) != 2 || !settings.ValidListenPort(args[1]) {
			return usageErr("hyla tailcat port <port>")
		}
		if !on {
			return errors.New("hylad does not listen over tailcat here: hyla tailcat enable")
		}
		set.SetTailcatListen(args[1])
		return settings.Save(set)
	case "rotate":
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		code, err := remote.RotateTailcatCode(ctx, remote.TailcatKeyPath())
		cancel()
		if err != nil {
			return err
		}
		// hylad starts its listener with the key again, which follow finds.
		set.TailcatRotate = !set.TailcatRotate
		if err := settings.Save(set); err != nil {
			return err
		}
		fmt.Println("tailcat://" + code)
		fmt.Fprintln(os.Stderr, "the old pair code no longer connects: the phone needs this one")
		return nil
	}
	return usageErr("hyla tailcat status|enable [port]|disable|port <port>|rotate|code")
}

// beaconTowerCmd implements `hyla beacontower` (`hyla beacon`): manage this
// machine's BeaconTower encrypted relay listener, relay address, optional
// access token, and pair code.
func beaconTowerCmd(args []string) error {
	set := settings.Load()
	_, on := set.BeaconTowerListen()
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}

	parseRelayFlags := func(rest []string) (relayArg string, tokenArg string, hasToken, clearToken bool, qr bool, err error) {
		for i := 0; i < len(rest); i++ {
			a := rest[i]
			switch {
			case a == "--token":
				if i+1 >= len(rest) {
					return "", "", false, false, false, usageErr("--token needs a value")
				}
				i++
				tokenArg = rest[i]
				hasToken = true
			case strings.HasPrefix(a, "--token="):
				tokenArg = strings.TrimPrefix(a, "--token=")
				hasToken = true
			case a == "--clear-token":
				clearToken = true
			case a == "--qr":
				qr = true
			case strings.HasPrefix(a, "-"):
				return "", "", false, false, false, usageErr("unknown flag %q", a)
			default:
				if relayArg != "" {
					return "", "", false, false, false, usageErr("unexpected argument %q", a)
				}
				relayArg = a
			}
		}
		return relayArg, tokenArg, hasToken, clearToken, qr, nil
	}

	switch sub {
	case "status", "show":
		if !on {
			fmt.Println("beacontower: not listening")
			fmt.Printf("  relay: %s\n", orDash(set.BeaconTowerRelay))
			fmt.Println("  hyla beacontower enable <relay-url> [--token <secret>]")
			fmt.Println("  hyla beacontower relay <relay-url> [--token <secret>]")
			return nil
		}
		fmt.Println("beacontower: listening on beacontower://")
		fmt.Printf("  relay: %s\n", orDash(set.BeaconTowerRelay))
		fmt.Printf("  relay token: %s\n", onOffWord(set.BeaconTowerToken != ""))
		if set.BeaconTowerRelay == "" {
			fmt.Println("  pair code: unset (run `hyla beacontower relay <relay-url>`)")
			return nil
		}
		if code, err := remote.LocalBeaconTowerAddress(set.BeaconTowerRelay, set.BeaconTowerToken); err == nil {
			fmt.Printf("  pair code: beacontower://%s\n", code)
		} else {
			fmt.Printf("  pair code: %v\n", err)
		}
		fmt.Fprintln(os.Stderr, "  the pair code is the secret: anyone with it can connect; hyla beacontower rotate makes a new one")
		return nil

	case "enable":
		relayArg, tokenArg, hasToken, clearToken, _, err := parseRelayFlags(args[1:])
		if err != nil {
			return err
		}
		if relayArg != "" {
			if !settings.ValidBeaconRelay(relayArg) {
				return usageErr("%q is not a BeaconTower relay (use host:port, tls://, ws://, or wss://)", relayArg)
			}
			set.BeaconTowerRelay = strings.TrimSpace(relayArg)
		}
		if clearToken {
			set.BeaconTowerToken = ""
		} else if hasToken {
			set.BeaconTowerToken = strings.TrimSpace(tokenArg)
		}
		set.SetBeaconTowerListen(true)
		if err := settings.Save(set); err != nil {
			return err
		}
		if set.BeaconTowerRelay == "" {
			fmt.Println("hylad will listen over beacontower once relay is set: hyla beacontower relay <relay-url>")
			return nil
		}
		code, err := remote.LocalBeaconTowerAddress(set.BeaconTowerRelay, set.BeaconTowerToken)
		if err != nil {
			return err
		}
		fmt.Println("beacontower://" + code)
		fmt.Fprintln(os.Stderr, "the pair code is the secret: anyone with it can connect; hyla beacontower rotate makes a new one")
		return nil

	case "disable":
		set.SetBeaconTowerListen(false)
		return settings.Save(set)

	case "relay":
		relayArg, tokenArg, hasToken, clearToken, _, err := parseRelayFlags(args[1:])
		if err != nil {
			return err
		}
		if relayArg == "" && !hasToken && !clearToken {
			return usageErr("hyla beacontower relay <relay-url> [--token <secret>|--clear-token]")
		}
		if relayArg != "" {
			if !settings.ValidBeaconRelay(relayArg) {
				return usageErr("%q is not a BeaconTower relay (use host:port, tls://, ws://, or wss://)", relayArg)
			}
			set.BeaconTowerRelay = strings.TrimSpace(relayArg)
		}
		if clearToken {
			set.BeaconTowerToken = ""
		} else if hasToken {
			set.BeaconTowerToken = strings.TrimSpace(tokenArg)
		}
		if err := settings.Save(set); err != nil {
			return err
		}
		fmt.Printf("beacontower relay: %s\n", orDash(set.BeaconTowerRelay))
		return nil

	case "code":
		_, _, _, _, drawQR, err := parseRelayFlags(args[1:])
		if err != nil {
			return err
		}
		if set.BeaconTowerRelay == "" {
			return errors.New("beacontower-relay is not configured: hyla beacontower relay <relay-url>")
		}
		code, err := remote.LocalBeaconTowerAddress(set.BeaconTowerRelay, set.BeaconTowerToken)
		if err != nil {
			return err
		}
		text := "beacontower://" + code
		if drawQR {
			printTerminalQR(text)
		}
		fmt.Println(text)
		fmt.Fprintln(os.Stderr, "the pair code is the secret: anyone with it can connect; hyla beacontower rotate makes a new one")
		if !on {
			fmt.Fprintln(os.Stderr, "hylad does not listen over beacontower here: hyla beacontower enable")
		}
		return nil

	case "rotate":
		code, err := remote.RotateBeaconTowerCode(remote.BeaconTowerKeyPath(), set.BeaconTowerRelay, set.BeaconTowerToken)
		if err != nil {
			return err
		}
		set.BeaconTowerRotate = !set.BeaconTowerRotate
		if err := settings.Save(set); err != nil {
			return err
		}
		if code != "" {
			fmt.Println("beacontower://" + code)
			fmt.Fprintln(os.Stderr, "the old pair code no longer connects: other machines need this one")
		} else {
			fmt.Println("beacontower keys rotated (set a relay with `hyla beacontower relay <url>` to generate a pair code)")
		}
		return nil
	}
	return usageErr("hyla beacontower status|enable [relay-url] [--token <secret>]|disable|relay <url>|code [--qr]|rotate")
}

func printTerminalQR(text string) {
	q, err := qrcode.Encode(text, qrcode.L)
	if err != nil {
		return
	}
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

// localPairCode is this machine's pair code, as hylad's key makes it.
func localPairCode() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return remote.LocalTailcatAddress(ctx)
}

// p2pCmd is `hyla p2p`: this machine on its network, at an address and
// with a password (the listener's token), which Hyla and HylaPocket take
// as the address and the token of a host. It never prints the tailcat
// pair code, which is hyla tailcat's.
func p2pCmd(args []string) error {
	set := settings.Load()
	spec, port, on := set.TCPListen()
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status", "show":
		switch {
		case !on:
			fmt.Println("p2p: not listening")
			fmt.Println("  hyla p2p enable [port]   listen on this machine's network (default port 7473)")
			fmt.Println("  hyla tailcat             the machines reached by a pair code")
		default:
			fmt.Printf("p2p: listening on %s (port %s)\n", spec, port)
			for _, addr := range p2pAddrs(spec, port) {
				fmt.Printf("  address: %s\n", addr)
			}
			if set.ListenToken == "" {
				fmt.Println("  password: none, which hylad refuses: hyla p2p password")
			} else {
				fmt.Printf("  password: %s\n", set.ListenToken)
				fmt.Fprintln(os.Stderr, "  the password is the secret: anyone with it can connect; hyla p2p password makes a new one")
			}
			fmt.Fprintln(os.Stderr, "  the code a phone scans: hyla pocket code")
		}
		return nil
	case "enable":
		if !on {
			set.SetTCPListen("", settings.TailcatListenPortDefault())
		}
		if len(args) > 1 {
			if !settings.ValidListenPort(args[1]) {
				return usageErr("%q is not a port", args[1])
			}
			set.SetTCPListen(specHost(spec), args[1])
		}
		if set.ListenToken == "" {
			token, err := remote.RandomToken()
			if err != nil {
				return err
			}
			set.ListenToken = token
		}
		if err := settings.Save(set); err != nil {
			return err
		}
		fmt.Println("hylad listens on this machine's network: hyla p2p status")
		return nil
	case "disable":
		set.SetTCPListen("", "")
		return settings.Save(set)
	case "port":
		if len(args) != 2 || !settings.ValidListenPort(args[1]) {
			return usageErr("hyla p2p port <port>")
		}
		if !on {
			return errors.New("hylad does not listen on this machine's network here: hyla p2p enable")
		}
		set.SetTCPListen(specHost(spec), args[1])
		return settings.Save(set)
	case "password":
		token, err := remote.RandomToken()
		if err != nil {
			return err
		}
		set.ListenToken = token
		if err := settings.Save(set); err != nil {
			return err
		}
		fmt.Println(token)
		fmt.Fprintln(os.Stderr, "the old password no longer connects: the hosts that have it need this one")
		return nil
	}
	return usageErr("hyla p2p status|enable [port]|disable|port <port>|password")
}

// specHost is a listener's bind address, "" for every one of this
// machine's.
func specHost(spec string) string {
	l, err := remote.ParseListen(spec)
	if err != nil {
		return ""
	}
	return l.Host
}

// p2pAddrs are the addresses a client of another machine reaches the
// tcp listener at: the one the listener names, or this machine's own,
// with the listener's port.
func p2pAddrs(spec, port string) []string {
	if host := specHost(spec); host != "" {
		if ip := net.ParseIP(host); ip == nil || !ip.IsUnspecified() {
			return []string{net.JoinHostPort(host, port)}
		}
	}
	var out []string
	for _, addr := range remote.LocalAddrs() {
		out = append(out, net.JoinHostPort(addr, port))
	}
	if out == nil {
		out = []string{net.JoinHostPort("", port)}
	}
	return out
}

// onOffWord is a setting on or off, as the commands say it.
func onOffWord(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// restartLocal restarts this machine's hylad into the Hyla installed: its
// service's, else one this executable starts.
func restartLocal() error {
	if service.Installed() {
		if err := service.Restart(); !errors.Is(err, service.ErrUnsupported) {
			return err
		}
	}
	if err := StopDaemon(); err != nil {
		return err
	}
	return client.StartDaemon()
}

// serveCmd is `hyla daemon serve`: hylad in the foreground, for a service
// manager, which never exits for being idle.
func serveCmd(args []string) error {
	var listen []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--listen" || a == "-l":
			if i+1 >= len(args) {
				return usageErr("%s needs a listener: tailcat://:7473", a)
			}
			i++
			listen = append(listen, args[i])
		case strings.HasPrefix(a, "--listen="):
			listen = append(listen, strings.TrimPrefix(a, "--listen="))
		default:
			return usageErr("hyla daemon serve [--listen tailcat://:7473]…")
		}
	}
	for _, l := range listen {
		if _, err := remote.ParseListen(l); err != nil {
			return usageErr("%v", err)
		}
	}
	return daemon.Run(daemon.Options{Serve: true, Listen: listen})
}
