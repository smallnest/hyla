package pocket

import "hyla/internal/remote"

// ValidPairCode reports whether s, a code URL's pair code without its
// scheme, is shaped as a tailcat pair code makes it: "tc", and base64url
// alone.
func ValidPairCode(s string) bool { return remote.LooksLikePairCode(s) }
