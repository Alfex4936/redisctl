package cmd

import (
	"testing"
)

// TestNormalizeClusterAddress tests the address normalization function
func TestNormalizeClusterAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal address",
			input:    "192.168.1.100:7001",
			expected: "192.168.1.100:7001",
		},
		{
			name:     "address with bus port",
			input:    "192.168.1.100:7001@17001",
			expected: "192.168.1.100:7001",
		},
		{
			name:     "localhost conversion",
			input:    "localhost:7001",
			expected: "127.0.0.1:7001",
		},
		{
			name:     "localhost with bus port",
			input:    "localhost:7001@17001",
			expected: "127.0.0.1:7001",
		},
		{
			name:     "empty host",
			input:    ":7001",
			expected: "127.0.0.1:7001",
		},
		{
			name:     "malformed address - no colon",
			input:    "192.168.1.100",
			expected: "192.168.1.100",
		},
		{
			name:     "malformed address - multiple colons",
			input:    "192.168.1.100:7001:extra",
			expected: "192.168.1.100:7001:extra",
		},
		{
			name:     "ipv6 localhost",
			input:    "[::1]:7001",
			expected: "[::1]:7001", // IPv6 not supported
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeClusterAddress(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeClusterAddress(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseNodeAddress tests the node address parsing function
func TestParseNodeAddress(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedHost string
		expectedPort string
		shouldError  bool
	}{
		{
			name:         "valid IP and port",
			input:        "192.168.1.100:7001",
			expectedHost: "192.168.1.100",
			expectedPort: "7001",
			shouldError:  false,
		},
		{
			name:         "localhost conversion",
			input:        "localhost:7001",
			expectedHost: "127.0.0.1",
			expectedPort: "7001",
			shouldError:  false,
		},
		{
			name:         "empty host conversion",
			input:        ":7001",
			expectedHost: "127.0.0.1",
			expectedPort: "7001",
			shouldError:  false,
		},
		{
			name:         "hostname and port",
			input:        "redis-master:6379",
			expectedHost: "redis-master",
			expectedPort: "6379",
			shouldError:  false,
		},
		{
			name:        "no port",
			input:       "192.168.1.100",
			shouldError: true,
		},
		{
			name:        "multiple colons",
			input:       "192.168.1.100:7001:extra",
			shouldError: true,
		},
		{
			name:        "empty string",
			input:       "",
			shouldError: true,
		},
		{
			name:        "invalid port - non-numeric",
			input:       "192.168.1.100:abc",
			shouldError: true,
		},
		{
			name:        "invalid port - empty",
			input:       "192.168.1.100:",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseNodeAddress(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("parseNodeAddress(%q) expected error, but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseNodeAddress(%q) unexpected error: %v", tt.input, err)
				return
			}

			if host != tt.expectedHost {
				t.Errorf("parseNodeAddress(%q) host = %q, want %q", tt.input, host, tt.expectedHost)
			}

			if port != tt.expectedPort {
				t.Errorf("parseNodeAddress(%q) port = %q, want %q", tt.input, port, tt.expectedPort)
			}
		})
	}
}

// TestFormatNumber tests the number formatting function
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "small number",
			input:    123,
			expected: "123",
		},
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "boundary - 999",
			input:    999,
			expected: "999",
		},
		{
			name:     "thousands - 1000",
			input:    1000,
			expected: "1.0K",
		},
		{
			name:     "thousands - 1500",
			input:    1500,
			expected: "1.5K",
		},
		{
			name:     "thousands - 999999",
			input:    999999,
			expected: "1000.0K",
		},
		{
			name:     "millions - 1000000",
			input:    1000000,
			expected: "1.0M",
		},
		{
			name:     "millions - 2500000",
			input:    2500000,
			expected: "2.5M",
		},
		{
			name:     "large millions",
			input:    123456789,
			expected: "123.5M",
		},
		{
			name:     "negative number",
			input:    -500,
			expected: "-500",
		},
		{
			name:     "negative thousands",
			input:    -1500,
			expected: "-1.5K",
		},
		{
			name:     "negative millions",
			input:    -2000000,
			expected: "-2.0M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNumber(tt.input)
			if result != tt.expected {
				t.Errorf("formatNumber(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGetNodeRole tests the node role determination function
func TestGetNodeRole(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected string
	}{
		{
			name:     "master node",
			flags:    []string{"master"},
			expected: "마스터",
		},
		{
			name:     "replica node",
			flags:    []string{"slave"},
			expected: "복제본",
		},
		{
			name:     "master with additional flags",
			flags:    []string{"master", "connected"},
			expected: "마스터",
		},
		{
			name:     "replica with additional flags",
			flags:    []string{"slave", "connected"},
			expected: "복제본",
		},
		{
			name:     "no role flags",
			flags:    []string{"connected"},
			expected: "알 수 없음",
		},
		{
			name:     "empty flags",
			flags:    []string{},
			expected: "알 수 없음",
		},
		{
			name:     "nil flags",
			flags:    nil,
			expected: "알 수 없음",
		},
		{
			name:     "multiple role flags - master first",
			flags:    []string{"master", "slave"}, // Edge case - shouldn't happen in practice
			expected: "마스터",
		},
		{
			name:     "multiple role flags - slave first",
			flags:    []string{"slave", "master"}, // Edge case - shouldn't happen in practice
			expected: "마스터",                       // Master MUST come first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNodeRole(tt.flags)
			if result != tt.expected {
				t.Errorf("getNodeRole(%v) = %q, want %q", tt.flags, result, tt.expected)
			}
		})
	}
}

// TestParseNodeFlags tests the unified node flags parsing function
func TestParseNodeFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagsStr string
		expected NodeFlags
	}{
		{
			name:     "master node",
			flagsStr: "master,connected",
			expected: NodeFlags{IsMaster: true},
		},
		{
			name:     "replica node",
			flagsStr: "slave,connected",
			expected: NodeFlags{IsReplica: true},
		},
		{
			name:     "failed master",
			flagsStr: "master,fail",
			expected: NodeFlags{IsMaster: true, IsFail: true},
		},
		{
			name:     "handshaking node",
			flagsStr: "handshake,noaddr",
			expected: NodeFlags{IsHandshake: true, IsNoAddr: true},
		},
		{
			name:     "empty flags",
			flagsStr: "",
			expected: NodeFlags{IsNoFlags: true},
		},
		{
			name:     "unknown flags",
			flagsStr: "connected,unknown",
			expected: NodeFlags{},
		},
		{
			name:     "complex flags",
			flagsStr: "master,connected,myself",
			expected: NodeFlags{IsMaster: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNodeFlags(tt.flagsStr)
			if result != tt.expected {
				t.Errorf("parseNodeFlags(%q) = %+v, want %+v", tt.flagsStr, result, tt.expected)
			}
		})
	}
}

// TestParseNodeFlagsSlice tests the slice version of flags parsing
func TestParseNodeFlagsSlice(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected NodeFlags
	}{
		{
			name:     "master node",
			flags:    []string{"master", "connected"},
			expected: NodeFlags{IsMaster: true},
		},
		{
			name:     "replica node",
			flags:    []string{"slave", "connected"},
			expected: NodeFlags{IsReplica: true},
		},
		{
			name:     "failed node",
			flags:    []string{"master", "fail"},
			expected: NodeFlags{IsMaster: true, IsFail: true},
		},
		{
			name:     "empty slice",
			flags:    []string{},
			expected: NodeFlags{IsNoFlags: true},
		},
		{
			name:     "nil slice",
			flags:    nil,
			expected: NodeFlags{IsNoFlags: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNodeFlagsSlice(tt.flags)
			if result != tt.expected {
				t.Errorf("parseNodeFlagsSlice(%v) = %+v, want %+v", tt.flags, result, tt.expected)
			}
		})
	}
}

// TestIsMasterNode tests the master node detection function
func TestIsMasterNode(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected bool
	}{
		{
			name:     "master node",
			flags:    []string{"master"},
			expected: true,
		},
		{
			name:     "replica node",
			flags:    []string{"slave"},
			expected: false,
		},
		{
			name:     "master with other flags",
			flags:    []string{"master", "connected"},
			expected: true,
		},
		{
			name:     "no master flag",
			flags:    []string{"connected"},
			expected: false,
		},
		{
			name:     "empty flags",
			flags:    []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMasterNode(tt.flags)
			if result != tt.expected {
				t.Errorf("isMasterNode(%v) = %v, want %v", tt.flags, result, tt.expected)
			}
		})
	}
}

// TestIsReplicaNode tests the replica node detection function
func TestIsReplicaNode(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected bool
	}{
		{
			name:     "replica node",
			flags:    []string{"slave"},
			expected: true,
		},
		{
			name:     "master node",
			flags:    []string{"master"},
			expected: false,
		},
		{
			name:     "replica with other flags",
			flags:    []string{"slave", "connected"},
			expected: true,
		},
		{
			name:     "no replica flag",
			flags:    []string{"connected"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReplicaNode(tt.flags)
			if result != tt.expected {
				t.Errorf("isReplicaNode(%v) = %v, want %v", tt.flags, result, tt.expected)
			}
		})
	}
}

// TestIsFailedNode tests the failed node detection function
func TestIsFailedNode(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected bool
	}{
		{
			name:     "failed node",
			flags:    []string{"master", "fail"},
			expected: true,
		},
		{
			name:     "healthy node",
			flags:    []string{"master", "connected"},
			expected: false,
		},
		{
			name:     "only fail flag",
			flags:    []string{"fail"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFailedNode(tt.flags)
			if result != tt.expected {
				t.Errorf("isFailedNode(%v) = %v, want %v", tt.flags, result, tt.expected)
			}
		})
	}
}

// TestGetNodeStatus tests the detailed status function
func TestGetNodeStatus(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected string
	}{
		{
			name:     "healthy master",
			flags:    []string{"master"},
			expected: "마스터",
		},
		{
			name:     "failed replica",
			flags:    []string{"slave", "fail"},
			expected: "복제본, 실패",
		},
		{
			name:     "handshaking node",
			flags:    []string{"handshake", "noaddr"},
			expected: "알 수 없음, 핸드셰이크 중, 주소 없음",
		},
		{
			name:     "complex status",
			flags:    []string{"master", "fail", "handshake"},
			expected: "마스터, 실패, 핸드셰이크 중",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNodeStatus(tt.flags)
			if result != tt.expected {
				t.Errorf("getNodeStatus(%v) = %q, want %q", tt.flags, result, tt.expected)
			}
		})
	}
}

// Benchmarks

// BenchmarkNormalizeClusterAddress benchmarks the address normalization function
func BenchmarkNormalizeClusterAddress(b *testing.B) {
	testCases := []string{
		"192.168.1.100:7001",
		"192.168.1.100:7001@17001",
		"localhost:7001",
		"localhost:7001@17001",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			normalizeClusterAddress(testCase)
		}
	}
}

// BenchmarkParseNodeAddress benchmarks the node address parsing function
func BenchmarkParseNodeAddress(b *testing.B) {
	testCases := []string{
		"192.168.1.100:7001",
		"localhost:7001",
		"redis-master:6379",
		":7001",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			parseNodeAddress(testCase)
		}
	}
}

// BenchmarkFormatNumber benchmarks the number formatting function
func BenchmarkFormatNumber(b *testing.B) {
	testCases := []int64{
		123,
		1500,
		999999,
		2500000,
		123456789,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			formatNumber(testCase)
		}
	}
}

// BenchmarkGetNodeRole benchmarks the node role determination function
func BenchmarkGetNodeRole(b *testing.B) {
	testCases := [][]string{
		{"master"},
		{"slave"},
		{"master", "connected"},
		{"slave", "connected", "noaddr"},
		{"connected", "noaddr"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			getNodeRole(testCase)
		}
	}
}

// BenchmarkParseNodeFlags benchmarks the flags parsing function
func BenchmarkParseNodeFlags(b *testing.B) {
	testCases := []string{
		"master,connected",
		"slave,connected",
		"master,fail",
		"handshake,noaddr",
		"master,connected,myself",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			parseNodeFlags(testCase)
		}
	}
}

// BenchmarkParseNodeFlagsSlice benchmarks the slice flags parsing function
func BenchmarkParseNodeFlagsSlice(b *testing.B) {
	testCases := [][]string{
		{"master", "connected"},
		{"slave", "connected"},
		{"master", "fail"},
		{"handshake", "noaddr"},
		{"master", "connected", "myself"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			parseNodeFlagsSlice(testCase)
		}
	}
}

// BenchmarkIsMasterNode benchmarks the master node detection function
func BenchmarkIsMasterNode(b *testing.B) {
	testCases := [][]string{
		{"master"},
		{"slave"},
		{"master", "connected"},
		{"slave", "connected", "noaddr"},
		{"connected", "noaddr"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			isMasterNode(testCase)
		}
	}
}

// BenchmarkGetNodeStatus benchmarks the detailed status function
func BenchmarkGetNodeStatus(b *testing.B) {
	testCases := [][]string{
		{"master"},
		{"slave", "fail"},
		{"handshake", "noaddr"},
		{"master", "fail", "handshake"},
		{"connected", "noaddr"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			getNodeStatus(testCase)
		}
	}
}

// BenchmarkFormatNumberMemoryAllocation benchmarks memory allocations
func BenchmarkFormatNumberMemoryAllocation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		formatNumber(123456789)
	}
}

// BenchmarkParseNodeAddressMemoryAllocation benchmarks memory allocations
func BenchmarkParseNodeAddressMemoryAllocation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parseNodeAddress("192.168.1.100:7001")
	}
}
