package config

// TCPMode controls how TCP traffic is captured transparently.
type TCPMode string

const (
	TCPModeOff    TCPMode = "off"
	TCPModeRedir  TCPMode = "redir"
	TCPModeTProxy TCPMode = "tproxy"
	TCPModeTun    TCPMode = "tun"
)

// UDPMode controls how UDP traffic is captured transparently.
type UDPMode string

const (
	UDPModeOff    UDPMode = "off"
	UDPModeTProxy UDPMode = "tproxy"
	UDPModeTun    UDPMode = "tun"
)

type ProxyMode = TCPMode // legacy alias

// ProxyModes holds the selected TCP and UDP transparent proxy modes.
type ProxyModes struct {
	TCP TCPMode
	UDP UDPMode
}

// NeedsTunInbound returns true if either TCP or UDP uses TUN mode.
func (pm ProxyModes) NeedsTunInbound() bool {
	return pm.TCP == TCPModeTun || pm.UDP == UDPModeTun
}

// IsSystemProxyOnly returns true when both TCP and UDP are off (system proxy mode).
func (pm ProxyModes) IsSystemProxyOnly() bool {
	return pm.TCP == TCPModeOff && pm.UDP == UDPModeOff
}

// NeedsTProxyInbound returns true if either TCP or UDP uses TProxy.
func (pm ProxyModes) NeedsTProxyInbound() bool {
	return pm.TCP == TCPModeTProxy || pm.UDP == UDPModeTProxy
}

// NeedsRedirectInbound returns true if TCP uses redirect mode.
func (pm ProxyModes) NeedsRedirectInbound() bool {
	return pm.TCP == TCPModeRedir
}
