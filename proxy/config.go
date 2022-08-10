package proxy

import (
	"github.com/sammy007/open-ethereum-pool/api"
	"github.com/sammy007/open-ethereum-pool/payouts"
	"github.com/sammy007/open-ethereum-pool/policy"
	"github.com/sammy007/open-ethereum-pool/storage"
)

type Config struct {
	Name                  string        `json:"name"`
	Proxy                 Proxy         `json:"proxy"`
	Api                   api.ApiConfig `json:"api"`
	Upstream              []Upstream    `json:"upstream"`
	UpstreamCheckInterval string        `json:"upstreamCheckInterval"`

	Threads int `json:"threads"`

	Coin  string         `json:"coin"`
	Redis storage.Config `json:"redis"`

	BlockUnlocker payouts.UnlockerConfig `json:"unlocker"`
	Payouts       payouts.PayoutsConfig  `json:"payouts"`
}

type Proxy struct {
	Enabled              bool           `json:"enabled"`
	Listen               string         `json:"listen"`
	BehindReverseProxy   bool           `json:"behindReverseProxy"`
	BlockRefreshInterval string         `json:"blockRefreshInterval"`
	Difficulty           float64        `json:"difficulty"`
	VarDiff              VarDiffOptions `json:"varDiff"`
	StateUpdateInterval  string         `json:"stateUpdateInterval"`
	HashrateExpiration   string         `json:"hashrateExpiration"`

	Policy policy.Config `json:"policy"`

	MaxFails    int64 `json:"maxFails"`
	HealthCheck bool  `json:"healthCheck"`

	Stratum Stratum `json:"stratum"`
}

type Stratum struct {
	Debug    bool   `json:"debug"`
	Enabled  bool   `json:"enabled"`
	Listen   string `json:"listen"`
	Timeout  string `json:"timeout"`
	MaxConn  int    `json:"maxConn"`
	TLS      bool   `json:"tls"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type Upstream struct {
	Name    string `json:"name"`
	Url     string `json:"url"`
	Timeout string `json:"timeout"`
}

type VarDiffOptions struct {
	Enabled         bool    `json:"enabled"`
	MinDiff         float64 `json:"minDiff"`
	MaxDiff         float64 `json:"maxDiff"`
	TargetTime      int64   `json:"targetTime"`
	RetargetTime    int64   `json:"retargetTime"`
	VariancePercent float64 `json:"variancePercent"`
	X2Mode          bool    `json:"x2mode"`
}
