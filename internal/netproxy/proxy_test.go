package netproxy

import (
	"net/http"
	"net/url"
	"os"
	"testing"
)

// The program the window starts has no proxy in its environment, and these
// tests are of what it does then: a shell that sets one would speak for it.
func TestMain(m *testing.M) {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"} {
		os.Unsetenv(k)
	}
	os.Exit(m.Run())
}

func request(t *testing.T, raw string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// A request off this machine goes through the proxy the system names, and one
// to this machine, or to the network it is on, does not: a pane's gateway or a
// NAS is reached directly, as the Mac's own exceptions say.
func TestProxyFollowsTheSystem(t *testing.T) {
	old := systemProxyFn
	systemProxyFn = func() string { return "http://127.0.0.1:7890" }
	t.Cleanup(func() { systemProxyFn = old })

	u, err := Proxy(request(t, "https://raw.githubusercontent.com/smallnest/hyla/main/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if u == nil || u.String() != "http://127.0.0.1:7890" {
		t.Errorf("an outside host goes through %v", u)
	}
	for _, raw := range []string{
		"http://127.0.0.1:3425/api/info",
		"http://localhost:8080/",
		"http://192.168.3.2:5000/",
		"http://10.0.0.5/",
		"http://172.16.1.1/",
		"http://nas.local/",
	} {
		if u, err := Proxy(request(t, raw)); err != nil || u != nil {
			t.Errorf("%s goes through %v (%v)", raw, u, err)
		}
	}
}

// No system proxy, no proxy: the request goes as net/http would send it.
func TestProxyWithoutASystemOne(t *testing.T) {
	old := systemProxyFn
	systemProxyFn = func() string { return "" }
	t.Cleanup(func() { systemProxyFn = old })
	if u, err := Proxy(request(t, "https://example.com/")); err != nil || u != nil {
		t.Errorf("a system without a proxy goes through %v (%v)", u, err)
	}
}

// A transport of the package's is one that asks Proxy, which the default
// transport does not: it reads the environment alone.
func TestTransportAsksTheSystem(t *testing.T) {
	old := systemProxyFn
	systemProxyFn = func() string { return "http://127.0.0.1:7890" }
	t.Cleanup(func() { systemProxyFn = old })
	tr := Transport()
	if tr.Proxy == nil {
		t.Fatal("the transport asks nothing")
	}
	u, err := tr.Proxy(request(t, "https://example.com/"))
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := url.Parse("http://127.0.0.1:7890"); u == nil || u.String() != want.String() {
		t.Errorf("the transport goes through %v", u)
	}
}

// The settings' proxy, where they name one, is the user's word on every
// download: it wins over the environment and the system, and this machine's
// own services are still reached directly.
func TestProxyFromTheSettings(t *testing.T) {
	oldSys := systemProxyFn
	systemProxyFn = func() string { return "http://127.0.0.1:1111" }
	SetOverride(func() string { return "http://127.0.0.1:2222" })
	t.Cleanup(func() { SetOverride(nil); systemProxyFn = oldSys })

	u, err := Proxy(request(t, "https://raw.githubusercontent.com/smallnest/hyla/main/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := url.Parse("http://127.0.0.1:2222"); u == nil || u.String() != want.String() {
		t.Errorf("a download goes through %v, not %v", u, want)
	}
	if u, err := Proxy(request(t, "http://127.0.0.1:3425/api/info")); err != nil || u != nil {
		t.Errorf("the pane's gateway goes through %v (%v)", u, err)
	}
}

// Install makes net/http's default transport ask Proxy, which is what a
// client without a transport of its own uses.
func TestInstall(t *testing.T) {
	old := http.DefaultTransport
	http.DefaultTransport = http.DefaultTransport.(*http.Transport).Clone()
	t.Cleanup(func() { http.DefaultTransport = old })
	Install()
	if http.DefaultTransport.(*http.Transport).Proxy == nil {
		t.Fatal("the default transport asks nothing")
	}
}
