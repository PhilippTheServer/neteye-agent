# neteye-agent

A lightweight Go agent that runs on every monitored Linux node. It collects
network interface state, live traffic counters, and the kernel routing table
every 5 seconds and streams them to neteye-center over a persistent WebSocket
connection with automatic reconnect.

## Requirements

- Linux kernel 3.10+ (uses netlink and `/proc/net/dev`)
- Go 1.24+ (to build from source)
- `CAP_NET_ADMIN` capability (required for netlink route reads — usually implicit
  when running as root or in a privileged container)

## Running

### Directly (simplest)

```bash
# Connect to a center at the default address
NETEYE_CENTER="ws://your-center-host:9090/ws" ./neteye-agent

# Or with a config file
./neteye-agent -config /etc/neteye/agent.yaml
```

### Build from source

```bash
go build -o neteye-agent ./cmd/agent
```

### Docker (for testing — host networking required)

```bash
docker build -t neteye-agent .
docker run --rm \
  --network host \
  --cap-add NET_ADMIN \
  -e NETEYE_CENTER="ws://localhost:9090/ws" \
  neteye-agent
```

> **Note:** `--network host` is mandatory. Without it the agent reports the
> container's virtual interface instead of the host's real interfaces.

## Configuration

Config can be provided as a YAML file and/or environment variables.
Environment variables always take precedence.

```yaml
center_url:       "ws://localhost:9090/ws"  # neteye-center agent endpoint
hostname:         ""                         # override auto-detected hostname
collect_interval: "5s"                       # how often to sample and push
reconnect_delay:  "5s"                       # initial delay before reconnect (backs off to 30 s)
agent_version:    "1.0.0"
```

| Environment variable | Overrides |
|----------------------|-----------|
| `NETEYE_CENTER`   | `center_url` |
| `NETEYE_HOSTNAME` | `hostname`   |

## What the agent collects

Every `collect_interval` the agent sends a single JSON update containing:

**Network interfaces** (via netlink `LinkList` + `AddrList`)
- Interface name, MAC address, operational state (up/down), MTU
- All assigned IPv4 and IPv6 addresses in CIDR notation
- Link-local addresses are skipped

**Traffic counters** (from `/proc/net/dev`)
- Bytes received/sent
- Packets received/sent
- Input/output errors
- Input/output drops

These are monotonic kernel counters. neteye-center computes the per-second
rates by comparing successive samples.

**Routing table** (via netlink `RouteList`, main table only)
- Destination CIDR, gateway IP, interface name, metric, flags

## Deployment

### systemd unit

Create `/etc/systemd/system/neteye-agent.service`:

```ini
[Unit]
Description=NetEye monitoring agent
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/neteye-agent -config /etc/neteye/agent.yaml
Restart=on-failure
RestartSec=5s
Environment=NETEYE_CENTER=ws://center.internal:9090/ws

[Install]
WantedBy=multi-user.target
```

```bash
systemctl enable --now neteye-agent
```

### Kubernetes DaemonSet

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: neteye-agent
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: neteye-agent
  template:
    metadata:
      labels:
        app: neteye-agent
    spec:
      hostNetwork: true      # read real host interfaces
      hostPID: true
      tolerations:
        - operator: Exists   # run on all nodes including control-plane
      containers:
        - name: neteye-agent
          image: neteye-agent:latest
          env:
            - name: NETEYE_CENTER
              value: "ws://neteye-center.monitoring.svc.cluster.local:9090/ws"
            - name: NETEYE_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          securityContext:
            capabilities:
              add: ["NET_ADMIN"]
```

## Project structure

```
neteye-agent/
├── cmd/agent/main.go           # Entry point
├── internal/
│   ├── config/config.go        # YAML + env-var config
│   └── collector/
│   │   ├── interfaces.go       # netlink interface + address collection
│   │   ├── metrics.go          # /proc/net/dev counter parsing
│   │   └── routes.go           # netlink routing table collection
│   └── client/client.go        # WebSocket session + reconnect loop
├── Dockerfile
└── go.mod
```
