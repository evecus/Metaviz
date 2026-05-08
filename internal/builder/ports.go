package builder

import (
	"fmt"
	"net"
)

// Ports holds listen ports for one session.
type Ports struct {
	DNS      int `json:"dns"`
	Mixed    int `json:"mixed"`
	Redirect int `json:"redirect"`
	TProxy   int `json:"tproxy"`
}

// DefaultPorts returns preferred fixed ports, falling back to any free port if occupied.
func DefaultPorts() Ports {
	return Ports{
		DNS:      preferPort(2053),
		Mixed:    preferPort(2081),
		Redirect: preferPort(2082),
		TProxy:   preferPort(2083),
	}
}

// preferPort returns preferred if free, otherwise finds any available port.
func preferPort(preferred int) int {
	if isPortFree(preferred) {
		return preferred
	}
	return anyFreePort()
}

func isPortFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

func anyFreePort() int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}
