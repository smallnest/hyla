package remote

import "testing"

func TestParseListen(t *testing.T) {
	for in, want := range map[string]Listen{
		"tailcat://":           {Kind: "tailcat", Port: ListenPort(), PocketPort: PocketPort},
		"tailcat://:7400":      {Kind: "tailcat", Port: "7400", PocketPort: PocketPort},
		"tcp://":               {Kind: "tcp", Port: ListenPort()},
		"tcp://:7400":          {Kind: "tcp", Port: "7400"},
		"tcp://0.0.0.0:7400":   {Kind: "tcp", Host: "0.0.0.0", Port: "7400"},
		"tcp://127.0.0.1:7297": {Kind: "tcp", Host: "127.0.0.1", Port: "7297"},
	} {
		got, err := ParseListen(in)
		if err != nil || got != want {
			t.Errorf("%q: %+v, %v", in, got, err)
		}
		if again, err := ParseListen(got.String()); err != nil || again != got {
			t.Errorf("%q reads back as %q: %+v, %v", in, got.String(), again, err)
		}
	}
	for _, bad := range []string{"", "udp://:1", "tcp://host/x", "tcp://:0", "tcp://:99999", "tailcat://host:7473", "tailnet://:7473"} {
		if l, err := ParseListen(bad); err == nil {
			t.Errorf("%q read as %+v", bad, l)
		}
	}
}
