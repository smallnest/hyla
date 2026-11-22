package remote

import (
	"net/http"
	"sync"
	"testing"
)

func TestParseListen(t *testing.T) {
	for in, want := range map[string]Listen{
		"beacontower://":       {Kind: "beacontower", PocketPort: PocketPort},
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
	for _, bad := range []string{"", "udp://:1", "tcp://host/x", "tcp://:0", "tcp://:99999", "tailcat://host:7473", "tailnet://:7473", "beacontower://host:7474", "beacontower://:7474", "beacontower:///foo"} {
		if l, err := ParseListen(bad); err == nil {
			t.Errorf("%q read as %+v", bad, l)
		}
	}
}

func TestServePocketConcurrentAccess(t *testing.T) {
	t.Cleanup(func() { ServePocket("7297", nil) })
	h := http.NotFoundHandler()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range 100 {
				if id%2 == 0 {
					ServePocket("7297", h)
				} else {
					_, _ = getPocketPortAndHandler()
					_ = getPocketHandler()
				}
			}
		}(i)
	}
	wg.Wait()
}
