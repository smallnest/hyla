package remote

import (
	"context"
	"net"

	"hyla/internal/beacontower"
)

// BeaconTowerRelay is hylad's BeaconTower host listener registered with a relay.
type BeaconTowerRelay struct {
	*beacontower.HostListener
	PairCode string // full beacontower://bt1_... address
}

// ListenBeaconTower loads (or creates) the key at path, registers this machine on
// relayAddr with optional relayToken, and returns an accepting BeaconTowerRelay.
func ListenBeaconTower(ctx context.Context, path, relayAddr, relayToken string) (*BeaconTowerRelay, error) {
	k, err := BeaconTowerKeyAt(path)
	if err != nil {
		return nil, err
	}
	kp, err := k.KeyPair()
	if err != nil {
		return nil, err
	}
	var onPocket func(net.Conn)
	if ph := getPocketHandler(); ph != nil {
		onPocket = func(c net.Conn) { servePocket(c) }
	}
	hl, err := beacontower.ListenHost(ctx, beacontower.HostConfig{
		RelayAddr:  relayAddr,
		RelayToken: relayToken,
		Keys:       kp,
		OnPocket:   onPocket,
	})
	if err != nil {
		return nil, err
	}
	code, err := hl.PairCode()
	if err != nil {
		_ = hl.Close()
		return nil, err
	}
	return &BeaconTowerRelay{
		HostListener: hl,
		PairCode:     code,
	}, nil
}
