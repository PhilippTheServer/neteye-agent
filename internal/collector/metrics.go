//go:build linux

package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Metrics holds raw monotonic counters for a network interface,
// read from /proc/net/dev.
type Metrics struct {
	BytesRecv   uint64
	BytesSent   uint64
	PacketsRecv uint64
	PacketsSent uint64
	ErrorsIn    uint64
	ErrorsOut   uint64
	DropsIn     uint64
	DropsOut    uint64
}

// CollectMetrics reads /proc/net/dev and returns a map from interface name
// to its current counter values.
func CollectMetrics() (map[string]Metrics, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("open /proc/net/dev: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only, close error not actionable

	result := make(map[string]Metrics)
	scanner := bufio.NewScanner(f)

	// Skip two header lines.
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colonIdx])
		if name == "lo" {
			continue
		}
		fields := strings.Fields(line[colonIdx+1:])
		if len(fields) < 16 {
			continue
		}
		// /proc/net/dev column order:
		// recv: bytes packets errs drop fifo frame compressed multicast
		// send: bytes packets errs drop fifo colls carrier compressed
		m := Metrics{
			BytesRecv:   parseU64(fields[0]),
			PacketsRecv: parseU64(fields[1]),
			ErrorsIn:    parseU64(fields[2]),
			DropsIn:     parseU64(fields[3]),
			BytesSent:   parseU64(fields[8]),
			PacketsSent: parseU64(fields[9]),
			ErrorsOut:   parseU64(fields[10]),
			DropsOut:    parseU64(fields[11]),
		}
		result[name] = m
	}
	return result, scanner.Err()
}

func parseU64(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}
