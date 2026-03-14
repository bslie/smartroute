package store

import "net"

func ipKey(ip net.IP) string {
	return ip.String()
}
