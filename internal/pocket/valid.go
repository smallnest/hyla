package pocket

import (
	"hyla/internal/beacontower"
	"hyla/internal/remote"
)

// ValidPairCode reports whether s, a code URL's pair code without its
// scheme, is a complete one: tailcat's "tc" and base64url alone, or
// BeaconTower's "bt1_" code, which decodes to a whole one.
func ValidPairCode(s string) bool {
	if remote.LooksLikePairCode(s) {
		return true
	}
	return beacontower.ValidPairCode(s)
}
