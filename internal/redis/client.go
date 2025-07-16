/*
TODO: 패키지 자체가 지금 redis 패키지랑 충돌할 수도 있어서 rename?
*/
package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ClusterNode represents a Redis cluster node
type ClusterNode struct {
	ID          string
	Address     string
	Flags       []string
	Master      string
	Slots       []SlotRange
	LinkState   string
	PingSent    int64
	PongRecv    int64
	ConfigEpoch int64
}

// SlotRange represents a range of hash slots
type SlotRange struct {
	Start int
	End   int
}

// ClusterManager manages Redis cluster operations
type ClusterManager struct {
	nodes    map[string]*redis.Client
	nodesMu  sync.RWMutex // Protects nodes map from concurrent access
	ctx      context.Context
	user     string
	password string
}

// NewClusterManager creates a new cluster manager
func NewClusterManager(user, password string) *ClusterManager {
	return &ClusterManager{
		nodes:    make(map[string]*redis.Client),
		ctx:      context.Background(),
		user:     user,
		password: password,
	}
}

// Connect connects to a Redis node
func (cm *ClusterManager) Connect(address string) (*redis.Client, error) {
	// First, try to get existing connection with read lock
	cm.nodesMu.RLock()
	if client, exists := cm.nodes[address]; exists {
		cm.nodesMu.RUnlock()
		return client, nil
	}
	cm.nodesMu.RUnlock()

	// If connection doesn't exist, acquire write lock to create new connection
	cm.nodesMu.Lock()
	defer cm.nodesMu.Unlock()

	// Double-check pattern: another goroutine might have created the connection
	if client, exists := cm.nodes[address]; exists {
		return client, nil
	}

	// Parse address
	host, port, err := parseAddress(address)
	if err != nil {
		return nil, fmt.Errorf("주소 파싱 실패: %w", err)
	}

	// Create Redis client options
	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Username: cm.user,
		Password: cm.password,
		DB:       0, // Redis Cluster only supports DB 0

		// Connection settings
		DialTimeout:  10 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,

		// Pool settings
		PoolSize:     10,
		MinIdleConns: 5,
	}

	client := redis.NewClient(opts)

	// Test connection
	if err := client.Ping(cm.ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis 연결 실패 (%s): %w", address, err)
	}

	// Thread-safe write to map
	cm.nodes[address] = client
	return client, nil
}

// GetClusterNodes retrieves cluster nodes information
func (cm *ClusterManager) GetClusterNodes(address string) ([]ClusterNode, error) {
	client, err := cm.Connect(address)
	if err != nil {
		return nil, err
	}

	result, err := client.ClusterNodes(cm.ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("CLUSTER NODES 명령 실패: %w", err)
	}

	return parseClusterNodes(result), nil
}

// GetClusterInfo retrieves cluster information
func (cm *ClusterManager) GetClusterInfo(address string) (map[string]string, error) {
	client, err := cm.Connect(address)
	if err != nil {
		return nil, err
	}

	result, err := client.ClusterInfo(cm.ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("CLUSTER INFO 명령 실패: %w", err)
	}

	return parseClusterInfo(result), nil
}

// Close closes all connections
func (cm *ClusterManager) Close() error {
	cm.nodesMu.Lock()
	defer cm.nodesMu.Unlock()

	for _, client := range cm.nodes {
		if err := client.Close(); err != nil {
			return err
		}
	}
	cm.nodes = make(map[string]*redis.Client)
	return nil
}

// Helper functions

func parseAddress(address string) (string, int, error) {
	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("잘못된 주소 형식: %s (예: localhost:6379)", address)
	}

	host := parts[0]
	if host == "" || host == "localhost" {
		host = "127.0.0.1"
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("잘못된 포트: %s", parts[1])
	}

	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("포트 범위 오류: %d (1-65535)", port)
	}

	return host, port, nil
}

func parseClusterNodes(output string) []ClusterNode {
	var nodes []ClusterNode
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		node := ClusterNode{
			ID:        fields[0],
			Address:   fields[1],
			Flags:     strings.Split(fields[2], ","),
			Master:    fields[3],
			LinkState: fields[7],
		}

		// Parse ping sent and pong received
		if len(fields) > 4 {
			if val, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
				node.PingSent = val
			}
		}
		if len(fields) > 5 {
			if val, err := strconv.ParseInt(fields[5], 10, 64); err == nil {
				node.PongRecv = val
			}
		}
		if len(fields) > 6 {
			if val, err := strconv.ParseInt(fields[6], 10, 64); err == nil {
				node.ConfigEpoch = val
			}
		}

		// Parse slots
		if len(fields) > 8 {
			for i := 8; i < len(fields); i++ {
				if slotRange := parseSlotRange(fields[i]); slotRange != nil {
					node.Slots = append(node.Slots, *slotRange)
				}
			}
		}

		nodes = append(nodes, node)
	}

	return nodes
}

func parseSlotRange(slot string) *SlotRange {
	if strings.Contains(slot, "-") {
		parts := strings.Split(slot, "-")
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				return &SlotRange{Start: start, End: end}
			}
		}
	} else {
		if num, err := strconv.Atoi(slot); err == nil {
			return &SlotRange{Start: num, End: num}
		}
	}
	return nil
}

func parseClusterInfo(output string) map[string]string {
	info := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			info[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return info
}
