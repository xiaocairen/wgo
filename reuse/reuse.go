package reuse

import (
	"context"
	"net"
)

var (
	Enabled      = false
	listenConfig = net.ListenConfig{
		Control: Control,
	}
)

// Listen listens at the given network and address. see net.Listen
// Returns a net.Listener created from a file discriptor for a socket
// with SO_REUSEPORT and SO_REUSEADDR option set.
func Listen(network, address string) (net.Listener, error) {
	return listenConfig.Listen(context.Background(), network, address)
}

// ListenPacket listens at the given network and address. see net.ListenPacket
// Returns a net.Listener created from a file discriptor for a socket
// with SO_REUSEPORT and SO_REUSEADDR option set.
func ListenPacket(network, address string) (net.PacketConn, error) {
	return listenConfig.ListenPacket(context.Background(), network, address)
}

// Dial dials the given network and address. see net.Dialer.Dial
// Returns a net.Conn created from a file discriptor for a socket
// with SO_REUSEPORT and SO_REUSEADDR option set.
func Dial(network, laddr, raddr string) (net.Conn, error) {
	nla, err := ResolveAddr(network, laddr)
	if err != nil {
		return nil, err
	}
	d := net.Dialer{
		Control:   Control,
		LocalAddr: nla,
	}
	return d.Dial(network, raddr)
}

// ResolveAddr parses given parameters to net.Addr.
func ResolveAddr(network, address string) (net.Addr, error) {
	switch network {
	case "ip", "ip4", "ip6":
		return net.ResolveIPAddr(network, address)
	case "tcp", "tcp4", "tcp6":
		return net.ResolveTCPAddr(network, address)
	case "udp", "udp4", "udp6":
		return net.ResolveUDPAddr(network, address)
	case "unix", "unixgram", "unixpacket":
		return net.ResolveUnixAddr(network, address)
	default:
		return nil, net.UnknownNetworkError(network)
	}
}

