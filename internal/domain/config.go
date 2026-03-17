package domain

import (
	"net"
	"strconv"
	"strings"
	"time"
)

// TunnelConfig — конфиг одного туннеля.
type TunnelConfig struct {
	Name           string `yaml:"name"`
	Endpoint       string `yaml:"endpoint"`
	PrivateKeyFile string `yaml:"private_key_file"`
	PeerPublicKey  string `yaml:"peer_public_key,omitempty"` // для wg setconf: один peer (endpoint + allowedips)
	RouteTable     int    `yaml:"route_table,omitempty"`
	FWMark         uint32 `yaml:"fwmark,omitempty"`
	IsDefault      bool   `yaml:"is_default,omitempty"`
}

// StaticRoute — статическое правило domain/CIDR -> tunnel.
type StaticRoute struct {
	Domain       string `yaml:"domain,omitempty"`
	CIDR         string `yaml:"cidr,omitempty"`
	IP           string `yaml:"ip,omitempty"`
	Tunnel       string `yaml:"tunnel"`
	TrafficClass string `yaml:"traffic_class,omitempty"`
}

// ProbeConfig — настройки проб.
type ProbeConfig struct {
	MaxProbesPerTick              int           `yaml:"max_probes_per_tick,omitempty"`
	MaxProbesPerDestinationPerMin int           `yaml:"max_probes_per_destination_per_minute,omitempty"`
	SignalTTL                     time.Duration `yaml:"signal_ttl,omitempty"`
	HTTPCheck                     bool          `yaml:"http_check,omitempty"`
	Timeout                       time.Duration `yaml:"timeout,omitempty"`
}

// GameModeConfig — game mode.
type GameModeConfig struct {
	Enabled   bool `yaml:"enabled,omitempty"`
	UDPSticky bool `yaml:"udp_sticky,omitempty"`
}

// QoSConfig — QoS (cake/htb).
type QoSConfig struct {
	Mode      string `yaml:"mode,omitempty"` // "cake" | "htb"
	CakeRTTMs int    `yaml:"cake_rtt_ms,omitempty"`
}

// PeerConfig — peer (пользователь) WireGuard-сервера для управления через CLI.
type PeerConfig struct {
	Name       string `yaml:"name"`
	PublicKey  string `yaml:"public_key"`
	AllowedIPs string `yaml:"allowed_ips"` // например 10.0.0.2/32
}

// WireGuardServerConfig — конфиг интерфейса WG-сервера и список peer'ов (пользователей).
type WireGuardServerConfig struct {
	Interface      string       `yaml:"interface"`                  // например wg0
	Address        string       `yaml:"address,omitempty"`          // адрес интерфейса, например 10.100.0.1/24
	ListenPort     int          `yaml:"listen_port,omitempty"`      // порт сервера, по умолчанию 51820
	PrivateKeyFile string       `yaml:"private_key_file,omitempty"` // путь к приватному ключу сервера
	PublicEndpoint string       `yaml:"public_endpoint,omitempty"`  // endpoint для клиентских конфигов (ip:port или domain:port)
	NATInterface   string       `yaml:"nat_interface,omitempty"`    // интерфейс для MASQUERADE (если пусто, определяется автоматически)
	PeersSubnet    string       `yaml:"peers_subnet,omitempty"`     // подсеть для автовыдачи IP при add, например 10.0.0.0/24
	Peers          []PeerConfig `yaml:"peers,omitempty"`
}

// Config — полная конфигурация.
type Config struct {
	TickIntervalMs              int                    `yaml:"tick_interval_ms,omitempty"`
	MinReconcileInterval        time.Duration          `yaml:"min_reconcile_interval,omitempty"`
	ClientSubnet                string                 `yaml:"client_subnet"` // immutable
	Tunnels                     []TunnelConfig         `yaml:"tunnels"`
	StaticRoutes                []StaticRoute          `yaml:"static_routes,omitempty"`
	Probe                       ProbeConfig            `yaml:"probe,omitempty"`
	GameMode                    GameModeConfig         `yaml:"game_mode,omitempty"`
	QoS                         QoSConfig              `yaml:"qos,omitempty"`
	ShutdownCleanup             string                 `yaml:"shutdown_cleanup,omitempty"` // full | preserve | rules-only
	DestTTL                     time.Duration          `yaml:"dest_ttl,omitempty"`
	DnsmasqLogPath              string                 `yaml:"dnsmasq_log_path,omitempty"` // путь к логу dnsmasq (log-queries) для подпитки DNS-кэша
	TunnelQuarantineCooldownSec int                    `yaml:"tunnel_quarantine_cooldown_sec,omitempty"`
	StickyCycles                int                    `yaml:"sticky_cycles,omitempty"`
	HysteresisWebPct            int                    `yaml:"hysteresis_web_pct,omitempty"`
	HysteresisBulkPct           int                    `yaml:"hysteresis_bulk_pct,omitempty"`
	HysteresisGamePct           int                    `yaml:"hysteresis_game_pct,omitempty"`
	StickyBonus                 int                    `yaml:"sticky_bonus,omitempty"`
	WireGuardServer             *WireGuardServerConfig `yaml:"wireguard_server,omitempty"`
}

// ConfigState — состояние конфига с generation.
type ConfigState struct {
	Current    *Config
	Generation uint64
	Applied    uint64
	LoadedAt   time.Time
	AppliedAt  time.Time
	Previous   *Config
}

// DefaultConfig возвращает конфиг с дефолтами.
func DefaultConfig() *Config {
	return &Config{
		TickIntervalMs:       2000,
		MinReconcileInterval: 500 * time.Millisecond,
		Probe: ProbeConfig{
			MaxProbesPerTick:              50,
			MaxProbesPerDestinationPerMin: 6,
			SignalTTL:                     120 * time.Second,
			Timeout:                       5 * time.Second,
			HTTPCheck:                     true, // иначе 403/geo-block не детектируются, переключение на другой туннель не срабатывает
		},
		DestTTL:                     120 * time.Second,
		TunnelQuarantineCooldownSec: 60,
		StickyCycles:                5,
		HysteresisWebPct:            15,
		HysteresisBulkPct:           25,
		HysteresisGamePct:           5,
		StickyBonus:                 50,
		ShutdownCleanup:             "full",
	}
}

// Validate проверяет конфиг (базовая валидация).
func (c *Config) Validate() error {
	if c.ClientSubnet == "" {
		return ErrInvalidConfig
	}
	if _, _, err := net.ParseCIDR(c.ClientSubnet); err != nil {
		return ErrInvalidConfig
	}
	if c.WireGuardServer != nil {
		ws := c.WireGuardServer
		if ws.Interface == "" {
			return ErrInvalidConfig
		}
		if ws.Address != "" {
			if _, _, err := net.ParseCIDR(ws.Address); err != nil {
				return ErrInvalidConfig
			}
		}
		if ws.PeersSubnet != "" {
			if _, _, err := net.ParseCIDR(ws.PeersSubnet); err != nil {
				return ErrInvalidConfig
			}
		}
		if ws.ListenPort < 0 || ws.ListenPort > 65535 {
			return ErrInvalidConfig
		}
		if ws.PublicEndpoint != "" {
			if _, _, err := net.SplitHostPort(ws.PublicEndpoint); err != nil {
				// Разрешаем форму "host:port" без [] для IPv6 только как явное ограничение.
				// Для простоты: если это не splitHostPort и нет единственного ":" с числовым портом — ошибка.
				parts := strings.Split(ws.PublicEndpoint, ":")
				if len(parts) != 2 {
					return ErrInvalidConfig
				}
				if _, convErr := strconv.Atoi(parts[1]); convErr != nil {
					return ErrInvalidConfig
				}
			}
		}
	}
	// Туннели могут быть пустыми: режим только управление пользователями (wireguard_server) или отложенный старт маршрутизации.
	return nil
}
