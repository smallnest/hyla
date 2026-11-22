package pocket

import "testing"

// The code URL HylaPocket reads is the one hyla prints, with the pair
// code's own characters alone, and the name escaped.
func TestCodeURLRoundTrip(t *testing.T) {
	in := Code{Pair: "tcQ4Lq8sG8GrCx9Tu9Px_Gq2IsWbZjWmMx1Qs3Vk1_2", Name: "studio one"}
	out, err := ParseCodeURL(in.CodeURL())
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("got %+v, want %+v", out, in)
	}
}

func TestParseCodeURLRefusesOtherText(t *testing.T) {
	for _, s := range []string{
		"",
		"https://example.com",
		"tailcat://tcabc",
		"hyla://pair?addr=192.0.2.10:7297&token=t",
		"hyla://pair?name=studio",
		"hyla://pair?code=abc",
	} {
		if _, err := ParseCodeURL(s); err == nil {
			t.Errorf("%q: want an error", s)
		}
	}
}

// A BeaconTower code goes in and comes out as itself: a phone that scans or
// pastes the line reads the code that reaches the host over its relay.
func TestCodeURLBeacon(t *testing.T) {
	in := Code{Pair: "beacontower://bt1_AZ3ApgE--dht6rZwMY738wvOW3ltNC3Nm7ZLmGCgnV8dJTHh7sPqRnP1Xcg6nM_kBC-ORYArZBdDZfMs1QYlkzIAEnJlbGF5LmV4YW1wbGU6NzQ3NAADdG9r", Name: "studio"}
	out, err := ParseCodeURL(in.CodeURL())
	if err != nil {
		t.Fatal(err)
	}
	want := Code{Pair: "bt1_AZ3ApgE--dht6rZwMY738wvOW3ltNC3Nm7ZLmGCgnV8dJTHh7sPqRnP1Xcg6nM_kBC-ORYArZBdDZfMs1QYlkzIAEnJlbGF5LmV4YW1wbGU6NzQ3NAADdG9r", Name: "studio"}
	if out != want {
		t.Fatalf("got %+v, want %+v", out, want)
	}
}
