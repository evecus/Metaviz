package ipfilter

// Mode controls how the IP filter operates.
type Mode string

const (
	ModeOff       Mode = "off"       // disabled
	ModeBlacklist Mode = "blacklist" // listed IPs are NOT proxied
	ModeWhitelist Mode = "whitelist" // only listed IPs are proxied
)

// Config is persisted to data/ipfilter.json.
type Config struct {
	Mode Mode   `json:"mode"`
	IPs  string `json:"ips"` // space-separated CIDR / plain IPs
}
