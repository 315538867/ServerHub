package sshpool

import (
	"fmt"
	"strconv"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

type MetricsResult struct {
	CPU    float64 `json:"cpu"`
	Mem    float64 `json:"mem"`
	Disk   float64 `json:"disk"`
	Load1  float64 `json:"load1"`
	Uptime int64   `json:"uptime"`
}

// metricsScript reads CPU (two /proc/stat samples 300ms apart), memory,
// disk, load average, and uptime from a Linux server.
const metricsScript = `
a=$(awk '/^cpu /{t=$2+$3+$4+$5+$6+$7+$8;i=$5;print t" "i}' /proc/stat)
sleep 0.3
b=$(awk '/^cpu /{t=$2+$3+$4+$5+$6+$7+$8;i=$5;print t" "i}' /proc/stat)
at=$(echo $a|cut -d' ' -f1); ai=$(echo $a|cut -d' ' -f2)
bt=$(echo $b|cut -d' ' -f1); bi=$(echo $b|cut -d' ' -f2)
echo "cpu $(awk "BEGIN{dt=$bt-$at;di=$bi-$ai;printf \"%.1f\",dt>0?(1-di/dt)*100:0}")"
mt=$(awk '/^MemTotal/{print $2}' /proc/meminfo)
ma=$(awk '/^MemAvailable/{print $2}' /proc/meminfo)
echo "mem $(awk "BEGIN{printf \"%.1f\",($mt-$ma)*100/$mt}")"
echo "disk $(df -P / | awk 'NR==2{print $5}' | tr -d '%')"
echo "load1 $(awk '{print $1}' /proc/loadavg)"
echo "uptime $(awk '{print int($1)}' /proc/uptime)"
`

func CollectMetrics(client *gossh.Client) (*MetricsResult, error) {
	out, err := Run(client, metricsScript)
	if err != nil {
		return nil, fmt.Errorf("collect metrics: %w", err)
	}

	result := &MetricsResult{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		val, _ := strconv.ParseFloat(parts[1], 64)
		switch parts[0] {
		case "cpu":
			result.CPU = val
		case "mem":
			result.Mem = val
		case "disk":
			result.Disk = val
		case "load1":
			result.Load1 = val
		case "uptime":
			result.Uptime = int64(val)
		}
	}
	return result, nil
}
