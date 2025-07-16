/*
TODO: TUI 나 GUI 면 sync.Pool?
*/

package cmd

import (
	"fmt"
	"redisctl/internal/redis"
	"strconv"
	"strings"
	"time"
)

// normalizeClusterAddress normalizes a cluster node address by removing bus port and converting localhost
func normalizeClusterAddress(address string) string {
	// Remove @busport if present (format: ip:port@busport)
	if idx := strings.Index(address, "@"); idx != -1 {
		address = address[:idx]
	}

	// Parse and normalize the address
	host, port, err := parseNodeAddress(address)
	if err != nil {
		return address // riginal
	}

	return fmt.Sprintf("%s:%s", host, port)
}

func parseNodeAddress(address string) (string, string, error) {
	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("잘못된 주소 형식: %s", address)
	}

	host := parts[0]
	if host == "" || host == "localhost" {
		host = "127.0.0.1"
	}

	port := parts[1]
	if _, err := strconv.Atoi(port); err != nil {
		return "", "", fmt.Errorf("잘못된 포트: %s", port)
	}

	return host, port, nil
}

func formatNumber(n int64) string {
	// 음수도 오나
	if n < 0 {
		return "-" + formatNumber(-n)
	}

	if n < 1000 {
		return strconv.FormatInt(n, 10)
	} else if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	} else {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
}

// 클러스터 상태 동적 확인
func waitForClusterStable(cm *redis.ClusterManager, node string, maxWait time.Duration) error {
	start := time.Now()
	for time.Since(start) < maxWait {
		info, err := cm.GetClusterInfo(node)
		if err == nil && info["cluster_state"] == "ok" {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("클러스터가 %v 내에 안정화되지 않았습니다", maxWait)
}

// NodeFlags represents parsed Redis node flags
type NodeFlags struct {
	IsMaster    bool
	IsReplica   bool
	IsFail      bool
	IsHandshake bool
	IsNoAddr    bool
	IsNoFlags   bool
}

// parseNodeFlags parses Redis cluster node flags into a structured format
func parseNodeFlags(flagsStr string) NodeFlags {
	if flagsStr == "" {
		return NodeFlags{IsNoFlags: true}
	}

	flags := strings.Split(flagsStr, ",")
	var nodeFlags NodeFlags

	for _, flag := range flags {
		switch strings.TrimSpace(flag) {
		case "master":
			nodeFlags.IsMaster = true
		case "slave":
			nodeFlags.IsReplica = true
		case "fail":
			nodeFlags.IsFail = true
		case "handshake":
			nodeFlags.IsHandshake = true
		case "noaddr":
			nodeFlags.IsNoAddr = true
		}
	}

	return nodeFlags
}

// parseNodeFlagsSlice parses Redis cluster node flags from a slice
func parseNodeFlagsSlice(flags []string) NodeFlags {
	if len(flags) == 0 {
		return NodeFlags{IsNoFlags: true}
	}

	var nodeFlags NodeFlags

	for _, flag := range flags {
		switch strings.TrimSpace(flag) {
		case "master":
			nodeFlags.IsMaster = true
		case "slave":
			nodeFlags.IsReplica = true
		case "fail":
			nodeFlags.IsFail = true
		case "handshake":
			nodeFlags.IsHandshake = true
		case "noaddr":
			nodeFlags.IsNoAddr = true
		}
	}

	return nodeFlags
}

// isMasterNode checks if node flags indicate a master node
func isMasterNode(flags []string) bool {
	nodeFlags := parseNodeFlagsSlice(flags)
	return nodeFlags.IsMaster
}

// isReplicaNode checks if node flags indicate a replica node
func isReplicaNode(flags []string) bool {
	nodeFlags := parseNodeFlagsSlice(flags)
	return nodeFlags.IsReplica
}

// isFailedNode checks if node flags indicate a failed node
func isFailedNode(flags []string) bool {
	nodeFlags := parseNodeFlagsSlice(flags)
	return nodeFlags.IsFail
}

// isHandshakeNode checks if node flags indicate a handshaking node
func isHandshakeNode(flags []string) bool {
	nodeFlags := parseNodeFlagsSlice(flags)
	return nodeFlags.IsHandshake
}

// getNodeRole returns the localized role string for display
func getNodeRole(flags []string) string {
	nodeFlags := parseNodeFlagsSlice(flags)

	if nodeFlags.IsMaster {
		return "마스터"
	}
	if nodeFlags.IsReplica {
		return "복제본"
	}
	return "알 수 없음"
}

// getNodeStatus returns a detailed status string including health indicators
func getNodeStatus(flags []string) string {
	nodeFlags := parseNodeFlagsSlice(flags)

	role := "알 수 없음"
	if nodeFlags.IsMaster {
		role = "마스터"
	} else if nodeFlags.IsReplica {
		role = "복제본"
	}

	var statusParts []string
	statusParts = append(statusParts, role)

	if nodeFlags.IsFail {
		statusParts = append(statusParts, "실패")
	}
	if nodeFlags.IsHandshake {
		statusParts = append(statusParts, "핸드셰이크 중")
	}
	if nodeFlags.IsNoAddr {
		statusParts = append(statusParts, "주소 없음")
	}

	return strings.Join(statusParts, ", ")
}
