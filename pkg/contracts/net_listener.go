package contracts

import "net"

type NetListener interface {
	Accept() (net.Conn, error)
	Close() error
	Addr() net.Addr
}
