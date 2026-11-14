package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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

// p2pCmd is `hyla p2p`: this machine reachable peer to peer, by a pair code
// rather than an SSH login, over Tailcat (WireGuard between the two, a DERP
// relay only to find each other). `hyla tailcat` is the same command, after
// the library; HylaPocket rides the same listener, and `hyla pocket` prints
// its code as a QR code.
func p2pCmd(args []string) error {
	set := settings.Load()
	spec, port, on := set.TailcatListen()
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status", "show":
		switch {
		case !set.Tailcat:
			fmt.Println("p2p: off (tailcat = false in the configuration)")
			fmt.Println("  turn it on with tailcat = true, then hyla p2p enable [port]")
		case !on:
			fmt.Println("p2p: on, not listening")
			fmt.Println("  hyla p2p enable [port]   listen for peers (default port 7473)")
			fmt.Println("  hyla pocket qr           the same code, as a QR code for a phone")
		default:
			fmt.Printf("p2p: listening on %s (port %s)\n", spec, port)
			if code, err := localPairCode(); err == nil {
				fmt.Printf("  pair code: %s\n", code)
			} else {
				fmt.Printf("  pair code: %v\n", err)
			}
			fmt.Fprintln(os.Stderr, "  the pair code is the secret: anyone with it can connect; hyla p2p rotate makes a new one")
			fmt.Fprintln(os.Stderr, "  HylaPocket scans the same code: hyla pocket qr")
		}
		return nil
	case "code":
		if !set.Tailcat {
			return errors.New("p2p is off: hyla p2p enable")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		code, err := remote.LocalTailcatAddress(ctx)
		cancel()
		if err != nil {
			return err
		}
		fmt.Println(code)
		fmt.Fprintln(os.Stderr, "the pair code is the secret: anyone with it can connect; hyla p2p rotate makes a new one")
		if !on {
			fmt.Fprintln(os.Stderr, "hylad does not listen for peers here: hyla p2p enable")
		} else {
			fmt.Fprintf(os.Stderr, "hylad listens on port %s (hyla p2p port sets it)\n", port)
		}
		return nil
	case "enable":
		if !set.Tailcat {
			return errors.New("p2p is off in the configuration (tailcat = false): hyla p2p needs tailcat = true")
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
		fmt.Println("hylad listens for peers: hyla p2p code")
		return nil
	case "disable":
		set.SetTailcatListen("")
		return settings.Save(set)
	case "port":
		if len(args) != 2 || !settings.ValidListenPort(args[1]) {
			return usageErr("hyla p2p port <port>")
		}
		if !on {
			return errors.New("hylad does not listen for peers here: hyla p2p enable")
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
	return usageErr("hyla p2p status|enable [port]|disable|port <port>|rotate|code")
}

// tailcatCmd is `hyla tailcat`: the same command as `hyla p2p`, after the
// library that carries it.
func tailcatCmd(args []string) error { return p2pCmd(args) }

// localPairCode is this machine's pair code, as hylad's key makes it.
func localPairCode() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return remote.LocalTailcatAddress(ctx)
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
