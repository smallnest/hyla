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

// tailcatCmd is `hyla tailcat`: this machine reachable over Tailcat, by a
// pair code rather than an SSH login (WireGuard between the two, a DERP
// relay only to find each other). HylaPocket rides the same listener, and
// `hyla pocket` prints its code as a QR code.
//
// `hyla p2p` is the same command for the machines reached by an address
// and a password: p2p and tailcat are two ways to reach a machine, not
// one under two names, so each command says its own name.
func tailcatCmd(args []string) error { return remoteCmd("tailcat", "p2p", args) }

// p2pCmd is `hyla p2p`, which says what the tailcat listener does, named
// p2p.
func p2pCmd(args []string) error { return remoteCmd("p2p", "tailcat", args) }

// remoteCmd is `hyla <name>`, the tailcat listener's command, which names
// the other way to reach a machine, other, where it says where to go.
func remoteCmd(name, other string, args []string) error {
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
			fmt.Printf("%s: off (tailcat = false in the configuration)\n", name)
			fmt.Printf("  turn it on with tailcat = true, then hyla %s enable [port]\n", name)
		case !on:
			fmt.Printf("%s: on, not listening\n", name)
			fmt.Printf("  hyla %s enable [port]   listen over tailcat (default port 7473)\n", name)
			fmt.Printf("  hyla %s             the machines reached by address and password\n", other)
			fmt.Println("  hyla pocket qr           the same code, as a QR code for a phone")
		default:
			fmt.Printf("%s: listening on %s (port %s)\n", name, spec, port)
			if code, err := localPairCode(); err == nil {
				fmt.Printf("  pair code: tailcat://%s\n", code)
			} else {
				fmt.Printf("  pair code: %v\n", err)
			}
			fmt.Fprintf(os.Stderr, "  the pair code is the secret: anyone with it can connect; hyla %s rotate makes a new one\n", name)
			fmt.Fprintln(os.Stderr, "  HylaPocket scans the same code: hyla pocket qr")
		}
		return nil
	case "code":
		if !set.Tailcat {
			return fmt.Errorf("tailcat is off: hyla %s enable", name)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		code, err := remote.LocalTailcatAddress(ctx)
		cancel()
		if err != nil {
			return err
		}
		fmt.Println("tailcat://" + code)
		fmt.Fprintf(os.Stderr, "the pair code is the secret: anyone with it can connect; hyla %s rotate makes a new one\n", name)
		if !on {
			fmt.Fprintf(os.Stderr, "hylad does not listen over tailcat here: hyla %s enable\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "hylad listens on port %s (hyla %s port sets it)\n", port, name)
		}
		return nil
	case "enable":
		if !set.Tailcat {
			return fmt.Errorf("tailcat is off in the configuration (tailcat = false): hyla %s needs tailcat = true", name)
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
		fmt.Printf("hylad listens over tailcat: hyla %s code\n", name)
		return nil
	case "disable":
		set.SetTailcatListen("")
		return settings.Save(set)
	case "port":
		if len(args) != 2 || !settings.ValidListenPort(args[1]) {
			return usageErr("hyla " + name + " port <port>")
		}
		if !on {
			return fmt.Errorf("hylad does not listen over tailcat here: hyla %s enable", name)
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
	return usageErr("hyla " + name + " status|enable [port]|disable|port <port>|rotate|code")
}

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
