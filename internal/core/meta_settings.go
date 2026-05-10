package core

// MetaSettings 是 MetaViz 的全局持久化设置，控制 mihomo 配置文件生成。
type MetaSettings struct {
	Inbound          InboundSettings          `json:"inbound"`
	Tun              TunSettings              `json:"tun"`
	Sniffer          SnifferSettings          `json:"sniffer"`
	Log              LogSettings              `json:"log"`
	ClashAPI         ClashAPISettings         `json:"clashAPI"`
	Misc             MiscSettings             `json:"misc"`
	Auth             AuthSettings             `json:"auth"`
	ScheduledRestart ScheduledRestartSettings `json:"scheduledRestart"`
}

type InboundSettings struct {
	MixedPort    int  `json:"mixedPort"`    // 默认 7890
	RedirectPort int  `json:"redirectPort"` // 默认 7892
	TProxyPort   int  `json:"tproxyPort"`   // 默认 7893
	DNSPort      int  `json:"dnsPort"`      // 默认 1053
	AllowLan     bool `json:"allowLan"`
	IPv6         bool `json:"ipv6"`
	FakeIP       bool `json:"fakeIP"` // 开启 fake-ip 模式（仅影响单节点/订阅模式的生成配置）
}

type TunSettings struct {
	Enable bool   `json:"enable"`
	Device string `json:"device"` // 默认 "Meta"
	Stack  string `json:"stack"`  // system/gvisor/mixed，默认 mixed
	MTU    int    `json:"mtu"`    // 默认 1500
}

type SnifferSettings struct {
	Enable              bool `json:"enable"`
	OverrideDestination bool `json:"overrideDestination"`
}

// LogSettings — mihomo 用 log-level 字符串，"silent" 表示禁用
type LogSettings struct {
	Level string `json:"level"` // silent/error/warning/info/debug/trace
}

type ClashAPISettings struct {
	Listen string `json:"listen"` // 默认 "0.0.0.0:9090"
	Secret string `json:"secret"`
	UI     string `json:"ui"`
	UIURL  string `json:"uiURL"`
}

type MiscSettings struct {
	FindProcessMode string `json:"findProcessMode"` // off/strict/always，默认 off
	UnifiedDelay    bool   `json:"unifiedDelay"`    // 默认 true
	TCPConcurrent   bool   `json:"tcpConcurrent"`   // 默认 true
	GeodataMode     bool   `json:"geodataMode"`     // 默认 false（用 mrs）
}

type AuthSettings struct {
	Enabled      bool   `json:"enabled"`
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
}

type ScheduledRestartSettings struct {
	Enabled bool   `json:"enabled"`
	Cron    string `json:"cron"`
}

func DefaultMetaSettings() MetaSettings {
	return MetaSettings{
		Inbound: InboundSettings{
			MixedPort:    7890,
			RedirectPort: 7892,
			TProxyPort:   7893,
			DNSPort:      1053,
			AllowLan:     false,
			IPv6:         false,
			FakeIP:       false,
		},
		Tun: TunSettings{
			Enable: false,
			Device: "Meta",
			Stack:  "mixed",
			MTU:    1500,
		},
		Sniffer: SnifferSettings{
			Enable:              true,
			OverrideDestination: true,
		},
		Log: LogSettings{
			Level: "warning",
		},
		ClashAPI: ClashAPISettings{
			Listen: "0.0.0.0:9090",
		},
		Misc: MiscSettings{
			FindProcessMode: "off",
			UnifiedDelay:    true,
			TCPConcurrent:   true,
			GeodataMode:     false,
		},
		Auth: AuthSettings{
			Enabled: true,
		},
		ScheduledRestart: ScheduledRestartSettings{
			Enabled: false,
			Cron:    "15 3 * * *",
		},
	}
}

func (ms MetaSettings) Filled() MetaSettings {
	d := DefaultMetaSettings()
	if ms.Inbound.MixedPort == 0 {
		ms.Inbound.MixedPort = d.Inbound.MixedPort
	}
	if ms.Inbound.RedirectPort == 0 {
		ms.Inbound.RedirectPort = d.Inbound.RedirectPort
	}
	if ms.Inbound.TProxyPort == 0 {
		ms.Inbound.TProxyPort = d.Inbound.TProxyPort
	}
	if ms.Inbound.DNSPort == 0 {
		ms.Inbound.DNSPort = d.Inbound.DNSPort
	}
	if ms.Tun.Device == "" {
		ms.Tun.Device = d.Tun.Device
	}
	if ms.Tun.Stack == "" {
		ms.Tun.Stack = d.Tun.Stack
	}
	if ms.Tun.MTU == 0 {
		ms.Tun.MTU = d.Tun.MTU
	}
	if ms.Log.Level == "" {
		ms.Log.Level = d.Log.Level
	}
	if ms.ClashAPI.Listen == "" {
		ms.ClashAPI.Listen = d.ClashAPI.Listen
	}
	if ms.Misc.FindProcessMode == "" {
		ms.Misc.FindProcessMode = d.Misc.FindProcessMode
	}
	if ms.ScheduledRestart.Cron == "" {
		ms.ScheduledRestart.Cron = d.ScheduledRestart.Cron
	}
	return ms
}

// InboundPorts 返回各监听端口（供 firewall 使用）。
func (ms MetaSettings) InboundPorts() (dns, mixed, redirect, tproxy int) {
	return ms.Inbound.DNSPort, ms.Inbound.MixedPort, ms.Inbound.RedirectPort, ms.Inbound.TProxyPort
}
