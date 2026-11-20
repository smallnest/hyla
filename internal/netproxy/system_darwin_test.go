//go:build darwin

package netproxy

import "testing"

// scutil's dictionary is what the Mac's settings say, HTTPS's proxy first.
func TestParseScutil(t *testing.T) {
	out := `<dictionary> {
  ExceptionsList : <array> {
    0 : 127.0.0.1
  }
  HTTPEnable : 1
  HTTPPort : 7891
  HTTPProxy : 127.0.0.1
  HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : 127.0.0.1
  ProxyAutoConfigEnable : 0
}`
	if got, want := parseScutil(out), "http://127.0.0.1:7890"; got != want {
		t.Errorf("the proxy is %q, not %q", got, want)
	}
	only := `<dictionary> {
  HTTPEnable : 1
  HTTPPort : 3128
  HTTPProxy : proxy.example.com
  HTTPSEnable : 0
}`
	if got, want := parseScutil(only), "http://proxy.example.com:3128"; got != want {
		t.Errorf("the proxy is %q, not %q", got, want)
	}
	none := `<dictionary> {
  HTTPEnable : 0
  HTTPSEnable : 0
}`
	if got := parseScutil(none); got != "" {
		t.Errorf("a Mac without a proxy says %q", got)
	}
}
