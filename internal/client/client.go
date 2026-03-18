// Package client manages the WebSocket connection to neteye-center,
// handles registration, sends periodic updates, and reconnects on failure.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"

	"github.com/neteye/agent/internal/collector"
	"github.com/neteye/agent/internal/config"
)

// agentMessage mirrors the center's models.AgentMessage for the wire protocol.
type agentMessage struct {
	Type     string      `json:"type"`
	Register *regPayload `json:"register,omitempty"`
	Update   *updPayload `json:"update,omitempty"`
}

type regPayload struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	AgentVersion string `json:"agent_version"`
}

type updPayload struct {
	Timestamp  time.Time   `json:"timestamp"`
	Interfaces []ifaceInfo `json:"interfaces"`
	Routes     []routeInfo `json:"routes"`
}

type ifaceInfo struct {
	Name      string       `json:"name"`
	MAC       string       `json:"mac"`
	State     string       `json:"state"`
	MTU       int          `json:"mtu"`
	SpeedMbps int64        `json:"speed_mbps"`
	Addresses []addrInfo   `json:"addresses"`
	Metrics   ifaceMetrics `json:"metrics"`
}

type addrInfo struct {
	Address string `json:"address"`
	Family  string `json:"family"`
}

type ifaceMetrics struct {
	BytesRecv   uint64 `json:"bytes_recv"`
	BytesSent   uint64 `json:"bytes_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	ErrorsIn    uint64 `json:"errors_in"`
	ErrorsOut   uint64 `json:"errors_out"`
	DropsIn     uint64 `json:"drops_in"`
	DropsOut    uint64 `json:"drops_out"`
}

type routeInfo struct {
	Destination   string `json:"destination"`
	Gateway       string `json:"gateway"`
	InterfaceName string `json:"interface_name"`
	Metric        int    `json:"metric"`
	Flags         string `json:"flags"`
}

// Client handles one persistent WebSocket session to the center.
type Client struct {
	cfg *config.Config
	log *slog.Logger
}

// New creates a Client.
func New(cfg *config.Config, log *slog.Logger) *Client {
	return &Client{cfg: cfg, log: log}
}

// Run connects to the center and keeps sending updates until ctx is done.
// On any error it waits ReconnectDelay and tries again.
func (c *Client) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := c.runSession(ctx); err != nil {
			c.log.Warn("session error, reconnecting",
				"err", err,
				"delay", c.cfg.ReconnectDelay)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(c.cfg.ReconnectDelay):
		}
	}
}

func (c *Client) runSession(ctx context.Context) error {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, _, err := websocket.DefaultDialer.DialContext(dialCtx, c.cfg.CenterURL, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.cfg.CenterURL, err)
	}
	defer conn.Close() //nolint:errcheck

	c.log.Info("connected to center", "url", c.cfg.CenterURL)

	// Send registration.
	reg := agentMessage{
		Type: "register",
		Register: &regPayload{
			Hostname:     c.cfg.Hostname,
			OS:           config.OS(),
			Arch:         config.Arch(),
			AgentVersion: c.cfg.AgentVersion,
		},
	}
	if err := writeJSON(conn, reg); err != nil {
		return fmt.Errorf("send register: %w", err)
	}
	c.log.Debug("registration sent", "hostname", c.cfg.Hostname)

	ticker := time.NewTicker(c.cfg.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			conn.WriteMessage(websocket.CloseMessage, //nolint:errcheck
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return nil

		case <-ticker.C:
			update, err := c.collectUpdate()
			if err != nil {
				c.log.Error("collect error", "err", err)
				continue
			}
			ifaceCount := 0
			if update.Update != nil {
				ifaceCount = len(update.Update.Interfaces)
			}
			c.log.Debug("sending update", "interfaces", ifaceCount)
			if err := writeJSON(conn, update); err != nil {
				return fmt.Errorf("send update: %w", err)
			}
		}
	}
}

func (c *Client) collectUpdate() (agentMessage, error) {
	ifaces, err := collector.CollectInterfaces()
	if err != nil {
		return agentMessage{}, fmt.Errorf("interfaces: %w", err)
	}

	rawMetrics, err := collector.CollectMetrics()
	if err != nil {
		return agentMessage{}, fmt.Errorf("metrics: %w", err)
	}

	routes, err := collector.CollectRoutes()
	if err != nil {
		c.log.Warn("routes collection failed, sending without routes", "err", err)
		// Non-fatal: continue without routes.
	}

	// Build interface list with merged metrics.
	var ifaceList []ifaceInfo
	for _, iface := range ifaces {
		info := ifaceInfo{
			Name:      iface.Name,
			MAC:       iface.MAC,
			State:     iface.State,
			MTU:       iface.MTU,
			SpeedMbps: iface.SpeedMbps,
		}
		for _, addr := range iface.Addresses {
			info.Addresses = append(info.Addresses, addrInfo{
				Address: addr.Address,
				Family:  addr.Family,
			})
		}
		if m, ok := rawMetrics[iface.Name]; ok {
			info.Metrics = ifaceMetrics{
				BytesRecv:   m.BytesRecv,
				BytesSent:   m.BytesSent,
				PacketsRecv: m.PacketsRecv,
				PacketsSent: m.PacketsSent,
				ErrorsIn:    m.ErrorsIn,
				ErrorsOut:   m.ErrorsOut,
				DropsIn:     m.DropsIn,
				DropsOut:    m.DropsOut,
			}
		}
		ifaceList = append(ifaceList, info)
	}

	// Build route list.
	var routeList []routeInfo
	for _, r := range routes {
		routeList = append(routeList, routeInfo{
			Destination:   r.Destination,
			Gateway:       r.Gateway,
			InterfaceName: r.InterfaceName,
			Metric:        r.Metric,
			Flags:         r.Flags,
		})
	}

	return agentMessage{
		Type: "update",
		Update: &updPayload{
			Timestamp:  time.Now().UTC(),
			Interfaces: ifaceList,
			Routes:     routeList,
		},
	}, nil
}

func writeJSON(conn *websocket.Conn, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}
