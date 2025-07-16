package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// GlobalConfig holds the global configuration
type GlobalConfig struct {
	User           string
	Password       string
	Debug          bool
	ConnectTimeout time.Duration
	CommandTimeout time.Duration
	MaxRetries     int
	PoolSize       int
	mutex          sync.RWMutex
}

var global = &GlobalConfig{}

// Init initializes the global configuration
func Init() {
	global.mutex.Lock()
	defer global.mutex.Unlock()

	// Set default values
	global.ConnectTimeout = 10 * time.Second
	global.CommandTimeout = 60 * time.Second
	global.MaxRetries = 3
	global.PoolSize = 10
	global.Debug = false

	// Load from environment variables if available
	loadFromEnvironment()

	// Enable debug mode if requested
	if global.Debug {
		fmt.Println("! Debug mode enabled")
	}
}

// SetAuth sets the authentication credentials
func SetAuth(user, password string) {
	global.mutex.Lock()
	defer global.mutex.Unlock()

	global.User = user
	global.Password = password
}

// GetAuth returns the authentication credentials
func GetAuth() (string, string) {
	global.mutex.RLock()
	defer global.mutex.RUnlock()

	return global.User, global.Password
}

// ValidateAuth checks if password is provided (required)
func ValidateAuth() error {
	global.mutex.RLock()
	defer global.mutex.RUnlock()

	if global.Password == "" {
		return fmt.Errorf("비밀번호가 필요합니다. --password 플래그를 사용하세요")
	}
	return nil
}

// loadFromEnvironment loads configuration from environment variables
func loadFromEnvironment() {
	// Load Redis credentials from environment
	if user := os.Getenv("REDIS_USER"); user != "" {
		global.User = user
	}
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		global.Password = password
	}

	// Load connection settings from environment
	if timeout := os.Getenv("REDIS_CONNECT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			global.ConnectTimeout = d
		}
	}
	if timeout := os.Getenv("REDIS_COMMAND_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			global.CommandTimeout = d
		}
	}
	if retries := os.Getenv("REDIS_MAX_RETRIES"); retries != "" {
		if r, err := strconv.Atoi(retries); err == nil && r > 0 {
			global.MaxRetries = r
		}
	}
	if poolSize := os.Getenv("REDIS_POOL_SIZE"); poolSize != "" {
		if p, err := strconv.Atoi(poolSize); err == nil && p > 0 {
			global.PoolSize = p
		}
	}

	// Enable debug mode
	if debug := os.Getenv("REDIS_DEBUG"); debug == "true" || debug == "1" {
		global.Debug = true
	}
}

// GetConnectTimeout returns the connection timeout
func GetConnectTimeout() time.Duration {
	global.mutex.RLock()
	defer global.mutex.RUnlock()
	return global.ConnectTimeout
}

// GetCommandTimeout returns the command timeout
func GetCommandTimeout() time.Duration {
	global.mutex.RLock()
	defer global.mutex.RUnlock()
	return global.CommandTimeout
}

// GetMaxRetries returns the maximum number of retries
func GetMaxRetries() int {
	global.mutex.RLock()
	defer global.mutex.RUnlock()
	return global.MaxRetries
}

// GetPoolSize returns the connection pool size
func GetPoolSize() int {
	global.mutex.RLock()
	defer global.mutex.RUnlock()
	return global.PoolSize
}

// IsDebugEnabled returns whether debug mode is enabled
func IsDebugEnabled() bool {
	global.mutex.RLock()
	defer global.mutex.RUnlock()
	return global.Debug
}

// SetDebug enables or disables debug mode
func SetDebug(enabled bool) {
	global.mutex.Lock()
	defer global.mutex.Unlock()
	global.Debug = enabled
}

// GetConfigSummary returns a summary of current configuration (for debug purposes)
func GetConfigSummary() string {
	global.mutex.RLock()
	defer global.mutex.RUnlock()

	return fmt.Sprintf(`Configuration:
  User: %s
  Password: %s
  Connect Timeout: %v
  Command Timeout: %v
  Max Retries: %d
  Pool Size: %d
  Debug: %t`,
		maskString(global.User),
		maskString(global.Password),
		global.ConnectTimeout,
		global.CommandTimeout,
		global.MaxRetries,
		global.PoolSize,
		global.Debug)
}

// maskString masks sensitive information for display
func maskString(s string) string {
	if s == "" {
		return "<not set>"
	}
	if len(s) <= 2 {
		return "***"
	}
	return s[:2] + "***"
}
