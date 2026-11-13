package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xingkaixin/mygo-native-ui/ui/qrcode"

	"hyla/internal/pocket"
	"hyla/internal/remote"
	"hyla/internal/settings"
)

// pocketCmd is `hyla hosts pocket`: this machine as a host for
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
	}
	return usageErr("hyla hosts pocket enable|disable|code|qr")
}

// pocketCode shows this machine's code: as the line HylaPocket takes
// pasted in, or with --qr as the QR code, drawn in the terminal.
func pocketCode(draw bool) error {
	s := settings.Load()
	if !s.Tailcat {
		return errors.New("tailcat is off, so HylaPocket cannot reach this machine: hyla hosts pocket enable")
	}
	if _, _, ok := s.TailcatListen(); !ok {
		return errors.New("hylad has no tailcat listener for HylaPocket to reach: hyla hosts pocket enable")
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
	fmt.Fprintln(os.Stderr, "the pair code in it reaches every session of this machine, to anyone who has it: hyla tailcat rotate makes another, and hyla hosts pocket disable stops HylaPocket")
	return nil
}
