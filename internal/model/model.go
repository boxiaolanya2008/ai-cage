package model

import "time"

type KernelMode string

const (
	KernelModeVerge   KernelMode = "verge"
	KernelModeSidecar KernelMode = "sidecar"
)

type ProxyMode string

const (
	ProxyModeRule   ProxyMode = "rule"
	ProxyModeGlobal ProxyMode = "global"
	ProxyModeDirect ProxyMode = "direct"
)

type VergeSettings struct {
	ControllerURL string `json:"controllerUrl"`
	Secret        string `json:"secret"`
}

type SidecarSettings struct {
	MihomoPath         string `json:"mihomoPath"`
	MihomoSHA256       string `json:"mihomoSha256"`
	ControllerPort     int    `json:"controllerPort"`
	MixedPort          int    `json:"mixedPort"`
	LogLevel           string `json:"logLevel"`
	ActiveSubscription string `json:"activeSubscription"`
}

type TUNSettings struct {
	Enabled             bool   `json:"enabled"`
	OnlyCagedApps       bool   `json:"onlyCagedApps"`
	Stack               string `json:"stack"`
	DeviceName          string `json:"deviceName"`
	AutoRoute           bool   `json:"autoRoute"`
	StrictRoute         bool   `json:"strictRoute"`
	AutoDetectInterface bool   `json:"autoDetectInterface"`
	IPv6                bool   `json:"ipv6"`
	PrimaryDNS          string `json:"primaryDns"`
	SecondaryDNS        string `json:"secondaryDns"`
}

type Settings struct {
	KernelMode KernelMode      `json:"kernelMode"`
	Verge      VergeSettings   `json:"verge"`
	Sidecar    SidecarSettings `json:"sidecar"`
	TUN        TUNSettings     `json:"tun"`
}

type SubscriptionKind string

const (
	SubscriptionKindRemote SubscriptionKind = "remote"
	SubscriptionKindVerge  SubscriptionKind = "verge"
)

type Subscription struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	URL       string           `json:"url"`
	Kind      SubscriptionKind `json:"kind"`
	UpdatedAt time.Time        `json:"updatedAt"`
}

type LaunchApp struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Path string   `json:"path"`
	Args []string `json:"args"`
}

type Version struct {
	Meta    bool   `json:"meta"`
	Version string `json:"version"`
}

type RunningConfig struct {
	Port               int    `json:"port"`
	SocksPort          int    `json:"socksPort"`
	MixedPort          int    `json:"mixedPort"`
	Mode               string `json:"mode"`
	LogLevel           string `json:"logLevel"`
	AllowLAN           bool   `json:"allowLan"`
	IPv6               bool   `json:"ipv6"`
	TUN                TUN    `json:"tun"`
	ExternalController string `json:"externalController,omitempty"`
}

type TUN struct {
	Enabled bool `json:"enable"`
}

type Traffic struct {
	Up        int64 `json:"up"`
	Down      int64 `json:"down"`
	UpTotal   int64 `json:"upTotal"`
	DownTotal int64 `json:"downTotal"`
}

type Connections struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Connections   []Connection `json:"connections"`
	Memory        uint64       `json:"memory"`
}

type Connection struct {
	ID       string         `json:"id"`
	Metadata ConnectionMeta `json:"metadata"`
	Upload   int64          `json:"upload"`
	Download int64          `json:"download"`
	Start    time.Time      `json:"start"`
	Chains   []string       `json:"chains"`
	Rule     string         `json:"rule"`
}

type ConnectionMeta struct {
	Network         string `json:"network"`
	Type            string `json:"type"`
	SourceIP        string `json:"sourceIP"`
	DestinationIP   string `json:"destinationIP"`
	SourcePort      string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Host            string `json:"host"`
	Process         string `json:"process"`
	ProcessPath     string `json:"processPath"`
}

type Proxy struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Now   string   `json:"now"`
	All   []string `json:"all"`
	Alive bool     `json:"alive"`
}

type Proxies struct {
	Proxies map[string]Proxy `json:"proxies"`
}

type DelayResult struct {
	Delay int `json:"delay"`
}

type Snapshot struct {
	Mode            KernelMode `json:"mode"`
	Running         bool       `json:"running"`
	CoreElevated    bool       `json:"coreElevated"`
	ControllerURL   string     `json:"controllerUrl"`
	Version         Version    `json:"version"`
	ProxyMode       ProxyMode  `json:"proxyMode"`
	TUNEnabled      bool       `json:"tunEnabled"`
	Traffic         Traffic    `json:"traffic"`
	ConnectionCount int        `json:"connectionCount"`
	Message         string     `json:"message"`
}

func DefaultSettings() Settings {
	return Settings{
		KernelMode: KernelModeVerge,
		Verge: VergeSettings{
			ControllerURL: "http://127.0.0.1:9097",
		},
		Sidecar: SidecarSettings{
			ControllerPort: 19097,
			MixedPort:      17897,
			LogLevel:       "info",
		},
		TUN: TUNSettings{
			Enabled:             false,
			OnlyCagedApps:       true,
			Stack:               "gvisor",
			DeviceName:          "ai-cage",
			AutoRoute:           true,
			StrictRoute:         true,
			AutoDetectInterface: true,
			PrimaryDNS:          "223.5.5.5",
			SecondaryDNS:        "119.29.29.29",
		},
	}
}
