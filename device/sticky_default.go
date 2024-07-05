//go:build !linux

package device

import (
	"github.com/darkit/wireguard/conn"
	"github.com/darkit/wireguard/rwcancel"
)

func (device *Device) startRouteListener(bind conn.Bind) (*rwcancel.RWCancel, error) {
	return nil, nil
}
