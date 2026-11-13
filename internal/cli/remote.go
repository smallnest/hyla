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

// tailcatCmd is `hyla tailcat`: this machine as a host for HylaPocket,
// which reaches it with its pair code instead of an SSH login.
func tailcatCmd(args []string) error {
	set := settings.Load()
	_, port, on := set.TailcatListen()
	sub := "code"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "code":
		if !set.Tailcat {
			return errors.New("Tailcat is off: hyla tailcat enable")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		code, err := remote.LocalTailcatAddress(ctx)
		cancel()
		if err != nil {
			return err
		}
		fmt.Println(code)
		fmt.Fprintln(os.Stderr, "the pair code is the secret: anyone with it can connect; hyla tailcat rotate makes a new one")
		if !on {
			fmt.Fprintln(os.Stderr, "hylad does not listen through Tailcat here: hyla tailcat enable")
		} else {
			fmt.Fprintf(os.Stderr, "hylad listens on port %s (hyla tailcat port sets it)\n", port)
		}
		return nil
	case "enable":
		if !set.Tailcat {
			return errors.New("Tailcat is off: turn it on first, with tailcat = true in the configuration")
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
		fmt.Println("hylad listens through Tailcat: hyla tailcat code")
		return nil
	case "disable":
		set.SetTailcatListen("")
		return settings.Save(set)
	case "port":
		if len(args) != 2 || !settings.ValidListenPort(args[1]) {
			return usageErr("hyla tailcat port <port>")
		}
		if !on {
			return errors.New("hylad does not listen through Tailcat here: hyla tailcat enable")
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
	return usageErr("hyla tailcat enable [port]|disable|port <port>|rotate|code")
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
