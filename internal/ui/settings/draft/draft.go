package draft

import (
	"time"

	"github.com/xingkaixin/mygo-native-ui/ui/qrcode"

	"hyla/internal/proto"
)

// The forms the root draws keep their drafts here, with the window's.

// TailcatDraft is the Tailcat form: this machine as a host over a relay,
// and its pair code.
type TailcatDraft struct {
	ListenPort string // this machine's tailcat listener's, while a host
	Relay      string
	Err        TailcatField // the field that failed, and why
	Code       string       // this machine's pair code, as hylad's key makes it
	CodeErr    string       // why there is none, as a failure
	QR         *qrcode.Code // Code as a QR code, for a phone to scan
	At         time.Time    // when the key was last read, or rotated
	Copied     time.Time
	Looking    bool
}

// P2PDraft is the P2P form: this machine on its network, by an address
// and a password.
type P2PDraft struct {
	Addr   string       // the network listener's address, "" for every
	Token  string       // its password, as listen-token keeps it
	Err    TailcatField // the field that failed, and why
	Copied time.Time
}

// TCHostDraft is the form that adds a machine by its tailcat pair code.
type TCHostDraft struct {
	Name, Code, Port string
	Err              string
}

// BeaconTowerDraft is the BeaconTower form: relay URL, optional access token,
// and this machine's self-contained pair code + QR code.
type BeaconTowerDraft struct {
	Relay   string
	Token   string
	Err     TailcatField
	Code    string
	CodeErr string
	QR      *qrcode.Code
	Copied  time.Time
	Loaded  bool
}

// BTHostDraft is the form that adds a machine by its BeaconTower pair code.
type BTHostDraft struct {
	Name, Code string
	Err        string
}

// HostDraft is the form that adds another machine: its name, its
// listener's address and its password, and what went wrong.
type HostDraft struct {
	Name, Addr, Token string
	Err               string
}

// FeishuDraft is the Feishu page's form, and the bot of this machine. The
// form takes what its fields hold, which a frame's elements do not keep
// beyond it: a field, or a row's button, reads it here.
type FeishuDraft struct {
	App     string
	AppSet  bool
	Secret  string
	Allow   string
	Dir     string
	Hosts   string
	Timeout string
	Stream  string
	Err     string            // why the last check failed
	Busy    bool              // a check with Feishu runs
	Filled  bool              // the fields hold the settings'
	Known   bool              // St was read
	St      proto.FeishuState // the bot's state, as hylad tells it
}

// TailcatField is a field of the Tailcat or P2P form that failed: which,
// as its key, and why.
type TailcatField struct {
	Key, Err string
}
