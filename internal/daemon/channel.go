package daemon

import "encoding/json"

// ChannelDriver abstracts an external messaging channel (such as the bundled
// hyla.feishu channel or future chat bridges) that drives non-terminal
// session turns and serves channel-specific control RPCs.
type ChannelDriver interface {
	ID() string
	Active() bool
	KeepAlive() bool
	Prompt(session, text string) error
	Call(method string, raw json.RawMessage) (result any, handled bool, err error)
	Stop()
}

func (d *Daemon) registerChannel(ch ChannelDriver) {
	if ch == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, existing := range d.channels {
		if existing.ID() == ch.ID() {
			return
		}
	}
	d.channels = append(d.channels, ch)
}

func (d *Daemon) ensureDefaultChannels() {
	d.mu.Lock()
	hasFeishu := false
	for _, ch := range d.channels {
		if ch.ID() == "hyla.feishu" {
			hasFeishu = true
			break
		}
	}
	if !hasFeishu {
		d.channels = append(d.channels, &feishuChannelDriver{d: d})
	}
	d.mu.Unlock()
}

func (d *Daemon) activeChannel() ChannelDriver {
	d.ensureDefaultChannels()
	d.mu.Lock()
	chs := append([]ChannelDriver(nil), d.channels...)
	d.mu.Unlock()
	for _, ch := range chs {
		if ch.Active() {
			return ch
		}
	}
	return nil
}

func (d *Daemon) channelActive() bool {
	return d.activeChannel() != nil
}

func (d *Daemon) channelKeepAlive() bool {
	d.ensureDefaultChannels()
	d.mu.Lock()
	chs := append([]ChannelDriver(nil), d.channels...)
	d.mu.Unlock()
	for _, ch := range chs {
		if ch.KeepAlive() {
			return true
		}
	}
	return false
}

func (d *Daemon) callChannel(method string, raw json.RawMessage) (any, bool, error) {
	d.ensureDefaultChannels()
	d.mu.Lock()
	chs := append([]ChannelDriver(nil), d.channels...)
	d.mu.Unlock()
	for _, ch := range chs {
		if res, handled, err := ch.Call(method, raw); handled {
			return res, true, err
		}
	}
	return nil, false, nil
}

func (d *Daemon) stopChannels() {
	d.mu.Lock()
	chs := append([]ChannelDriver(nil), d.channels...)
	d.mu.Unlock()
	for _, ch := range chs {
		ch.Stop()
	}
}
