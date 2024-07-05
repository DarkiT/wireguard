package main

import (
	"context"
	"errors"
	"github.com/darkit/wireguard/conn"
	"github.com/darkit/wireguard/device"
	"github.com/darkit/wireguard/tun/netstack"
	"github.com/darkit/wireguard/tun/netstack/examples/socks5"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"log"
	"net"
	"net/netip"
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

	if err := ServeSocks5(tnet, []byte("192.168.1.1"), ":1080", "119.29.29.29:53"); err != nil {
		log.Panic(err)
	}
}

func ServeSocks5(ipStack *netstack.Net, selfIp []byte, bindAddr, dnsServer string) error {
	if bindAddr == "" {
		bindAddr = ":1080"
	}
	resolver := socks5.DNSResolver{}
	if dnsServer != "" {
		resolver = socks5.DNSResolver{
			Resolver: net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					return ipStack.DialContext(ctx, network, dnsServer)
				},
			},
		}
	}
	server := socks5.Server{
		Resolver: &resolver,
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {

			log.Printf("socks dial: %s", addr)

			if network != "tcp" {
				return nil, errors.New("only support tcp")
			}

			addrPort, err := netip.ParseAddrPort(addr)
			if err != nil {
				return nil, errors.New("parse AddrPort failed: " + err.Error())
			}

			addrTarget := tcpip.FullAddress{
				NIC:  defaultNIC,
				Port: addrPort.Port(),
				Addr: tcpip.AddrFromSlice(addrPort.Addr().AsSlice()),
			}

			bind := tcpip.FullAddress{
				NIC:  defaultNIC,
				Addr: tcpip.AddrFromSlice(selfIp),
			}

			return gonet.DialTCPWithBind(context.Background(), ipStack.Stack(), bind, addrTarget, header.IPv4ProtocolNumber)
		},
	}

	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return err
	}

	log.Printf(">>>SOCKS5 SERVER listening on<<<: " + bindAddr)

	return server.Serve(listener)
}
