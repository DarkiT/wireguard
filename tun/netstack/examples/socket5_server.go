package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/darkit/wireguard/conn"
	"github.com/darkit/wireguard/device"
	"github.com/darkit/wireguard/tun/netstack"
	"github.com/darkit/wireguard/tun/netstack/examples/socks5"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const defaultNIC tcpip.NICID = 1

func main() {
	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("192.168.4.29")},
		[]netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("8.8.4.4")},
		1420,
	)
	if err != nil {
		log.Panic(err)
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelVerbose, ""))
	dev.IpcSet(`private_key=003ed5d73b55806c30de3f8a7bdab38af13539220533055e635690b8b87ad641
listen_port=58120
public_key=f928d4f6c1b86c12f2562c10b07c555c5c57fd00f59e90c8d8d88767271cbf7c
allowed_ip=192.168.4.28/32
persistent_keepalive_interval=25
`)
	dev.Up()

	if err := ServeSocks5(tnet.Stack(), []byte("192.168.1.1"), ":1080"); err != nil {
		log.Panic(err)
	}
}

func ServeSocks5(ipStack *stack.Stack, selfIp []byte, bindAddr string) error {
	if bindAddr == "" {
		bindAddr = ":1080"
	}
	server := socks5.Server{
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {

			log.Printf("socks dial: %s", addr)

			if network != "tcp" {
				return nil, errors.New("only support tcp")
			}

			parts := strings.Split(addr, ":")
			target, err := net.ResolveIPAddr("ip", parts[0])
			if err != nil {
				return nil, errors.New("resolve ip addr failed: " + parts[0])
			}

			port, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, errors.New("invalid port: " + parts[1])
			}

			addrTarget := tcpip.FullAddress{
				NIC:  defaultNIC,
				Port: uint16(port),
				Addr: tcpip.AddrFromSlice(target.IP),
			}

			bind := tcpip.FullAddress{
				NIC:  defaultNIC,
				Addr: tcpip.AddrFromSlice(selfIp),
			}

			return gonet.DialTCPWithBind(context.Background(), ipStack, bind, addrTarget, header.IPv4ProtocolNumber)
		},
	}

	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return err
	}

	log.Printf(">>>SOCKS5 SERVER listening on<<<: " + bindAddr)

	return server.Serve(listener)
}

func parseNetIPAddr(addrStr string) (*net.IPAddr, uint16, error) {
	parts := strings.Split(addrStr, ":")
	target, err := net.ResolveIPAddr("ip", parts[0])
	if err != nil {
		return nil, 0, errors.New("resolve ip addr failed: " + parts[0])
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, 0, errors.New("invalid port: " + parts[1])
	}
	return target, uint16(port), nil
}
