package firewall

import (
	"net"
)

// SyncLocalIPs adds all current local interface CIDRs to the nftables sets.
// Called once when the firewall rules are applied.
func SyncLocalIPs() {
	cidrs, err := localCIDRs()
	if err != nil {
		return
	}
	for _, cidr := range cidrs {
		AddInterfaceIP(cidr)
	}
}

func localCIDRs() ([]string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			out = append(out, ipnet.String())
		}
	}
	return out, nil
}
