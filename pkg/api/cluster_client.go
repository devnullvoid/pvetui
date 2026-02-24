// Package api provides Proxmox API client functionality.
package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

// Default health check configuration for cluster mode.
const (
	// DefaultHealthCheckInterval is the default interval between health check pings.
	DefaultHealthCheckInterval = 30 * time.Second

	// DefaultHealthCheckTimeout is the timeout for a single health check request.
	DefaultHealthCheckTimeout = 10 * time.Second

	// healthCheckPath is the lightweight Proxmox API endpoint used for health checks.
	healthCheckPath = "/version"
)

// ClusterClient manages a group of Proxmox nodes from the same cluster,
// connecting to one at a time with automatic failover. Unlike GroupClientManager
// which connects to ALL profiles simultaneously (aggregate mode), ClusterClient
// maintains a single active connection and fails over to the next candidate
// when the active node becomes unreachable.
type ClusterClient struct {
	groupName     string
	candidates    []ProfileEntry // ordered list of failover candidates
	activeClient  *Client
	activeProfile string
	activeIndex   int
	logger        interfaces.Logger
	cache         interfaces.Cache

	// Health check state
	healthTicker *time.Ticker
	healthStop   chan struct{}
	healthInterval time.Duration

	// Failover callback — called after a successful failover.
	// Parameters: oldProfile, newProfile.
	onFailover func(oldProfile, newProfile string)

	mu sync.RWMutex
}

// NewClusterClient creates a new cluster client manager for HA failover mode.
// The candidates list defines the failover order: first entry is preferred,
// subsequent entries are tried in order when the active node is unreachable.
func NewClusterClient(
	groupName string,
	logger interfaces.Logger,
	cache interfaces.Cache,
) *ClusterClient {
	return &ClusterClient{
		groupName:      groupName,
		logger:         logger,
		cache:          cache,
		activeIndex:    -1,
		healthInterval: DefaultHealthCheckInterval,
	}
}

// Initialize connects to the first available candidate in order.
// It tries each candidate sequentially until one succeeds.
// Returns an error only if ALL candidates fail to connect.
func (cc *ClusterClient) Initialize(ctx context.Context, profiles []ProfileEntry) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if len(profiles) == 0 {
		return fmt.Errorf("no profiles provided for cluster group '%s'", cc.groupName)
	}

	cc.candidates = profiles
	cc.activeClient = nil
	cc.activeProfile = ""
	cc.activeIndex = -1

	// Try each candidate in order
	var lastErr error
	for i, entry := range profiles {
		cc.logger.Info("[CLUSTER] Trying candidate %d/%d: %s", i+1, len(profiles), entry.Name)

		client, err := NewClient(entry.Config,
			WithLogger(cc.logger),
			WithCache(cc.cache),
		)
		if err != nil {
			cc.logger.Error("[CLUSTER] Failed to connect to %s: %v", entry.Name, err)
			lastErr = fmt.Errorf("profile %s: %w", entry.Name, err)
			continue
		}

		cc.activeClient = client
		cc.activeProfile = entry.Name
		cc.activeIndex = i
		cc.logger.Info("[CLUSTER] Connected to %s (candidate %d/%d)",
			entry.Name, i+1, len(profiles))
		return nil
	}

	return fmt.Errorf("failed to connect to any candidate in cluster group '%s': %w",
		cc.groupName, lastErr)
}

// GetActiveClient returns the currently active API client.
// This is the primary method used by the app — it returns a single *Client
// that can be used identically to a regular single-profile connection.
func (cc *ClusterClient) GetActiveClient() *Client {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.activeClient
}

// GetActiveProfileName returns the name of the currently active profile.
func (cc *ClusterClient) GetActiveProfileName() string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.activeProfile
}

// GetGroupName returns the cluster group name.
func (cc *ClusterClient) GetGroupName() string {
	return cc.groupName
}

// SetOnFailover registers a callback that fires after a successful failover.
// The callback receives the old and new profile names.
func (cc *ClusterClient) SetOnFailover(callback func(oldProfile, newProfile string)) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.onFailover = callback
}

// SetHealthCheckInterval configures the interval between health check pings.
// Must be called before StartHealthCheck.
func (cc *ClusterClient) SetHealthCheckInterval(interval time.Duration) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.healthInterval = interval
}

// Failover attempts to connect to the next available candidate.
// It tries candidates in round-robin order starting from the one after
// the currently active profile. Returns an error if no candidate is reachable.
func (cc *ClusterClient) Failover(ctx context.Context) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	return cc.failoverLocked(ctx)
}

// failoverLocked performs the failover while holding the write lock.
func (cc *ClusterClient) failoverLocked(ctx context.Context) error {
	if len(cc.candidates) == 0 {
		return fmt.Errorf("no candidates available for failover in cluster group '%s'", cc.groupName)
	}

	oldProfile := cc.activeProfile
	numCandidates := len(cc.candidates)

	cc.logger.Info("[CLUSTER] Initiating failover from %s", oldProfile)

	// Try each candidate starting from the next one in order
	for attempt := 1; attempt <= numCandidates; attempt++ {
		idx := (cc.activeIndex + attempt) % numCandidates
		entry := cc.candidates[idx]

		cc.logger.Info("[CLUSTER] Failover attempt %d/%d: trying %s",
			attempt, numCandidates, entry.Name)

		client, err := NewClient(entry.Config,
			WithLogger(cc.logger),
			WithCache(cc.cache),
		)
		if err != nil {
			cc.logger.Error("[CLUSTER] Failover to %s failed: %v", entry.Name, err)
			continue
		}

		cc.activeClient = client
		cc.activeProfile = entry.Name
		cc.activeIndex = idx
		cc.logger.Info("[CLUSTER] Failover successful: %s -> %s", oldProfile, entry.Name)

		// Notify callback outside the lock to avoid deadlocks
		if cc.onFailover != nil {
			callback := cc.onFailover
			newProfile := entry.Name
			go callback(oldProfile, newProfile)
		}

		return nil
	}

	return fmt.Errorf("failover exhausted: no reachable candidate in cluster group '%s'", cc.groupName)
}

// StartHealthCheck begins periodic health checking of the active node.
// If the active node becomes unreachable, it automatically triggers failover.
// The health check pings the /api2/json/version endpoint which is lightweight.
func (cc *ClusterClient) StartHealthCheck() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Stop any existing health check
	cc.stopHealthCheckLocked()

	cc.healthStop = make(chan struct{})
	cc.healthTicker = time.NewTicker(cc.healthInterval)

	go cc.healthCheckLoop()

	cc.logger.Info("[CLUSTER] Health check started (interval: %s)", cc.healthInterval)
}

// StopHealthCheck stops the periodic health checking.
func (cc *ClusterClient) StopHealthCheck() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.stopHealthCheckLocked()
}

// stopHealthCheckLocked stops the health check while holding the lock.
func (cc *ClusterClient) stopHealthCheckLocked() {
	if cc.healthStop != nil {
		close(cc.healthStop)
		cc.healthStop = nil
	}
	if cc.healthTicker != nil {
		cc.healthTicker.Stop()
		cc.healthTicker = nil
	}
}

// healthCheckLoop runs the periodic health check in a goroutine.
func (cc *ClusterClient) healthCheckLoop() {
	cc.mu.RLock()
	ticker := cc.healthTicker
	stop := cc.healthStop
	cc.mu.RUnlock()

	if ticker == nil || stop == nil {
		return
	}

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			cc.performHealthCheck()
		}
	}
}

// performHealthCheck pings the active node and triggers failover if unreachable.
func (cc *ClusterClient) performHealthCheck() {
	cc.mu.RLock()
	client := cc.activeClient
	profile := cc.activeProfile
	cc.mu.RUnlock()

	if client == nil {
		return
	}

	// Use a short timeout for health checks
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHealthCheckTimeout)
	defer cancel()

	// Ping the version endpoint — lightweight, no auth issues
	var result map[string]interface{}
	err := client.httpClient.GetWithRetry(ctx, healthCheckPath, &result, 1)
	if err != nil {
		cc.logger.Error("[CLUSTER] Health check failed for %s: %v", profile, err)
		cc.logger.Info("[CLUSTER] Triggering automatic failover")

		// Trigger failover
		cc.mu.Lock()
		failoverErr := cc.failoverLocked(ctx)
		cc.mu.Unlock()

		if failoverErr != nil {
			cc.logger.Error("[CLUSTER] Automatic failover failed: %v", failoverErr)
		}
		return
	}

	cc.logger.Debug("[CLUSTER] Health check OK for %s", profile)
}

// Close stops health checks and cleans up resources.
func (cc *ClusterClient) Close() {
	cc.StopHealthCheck()

	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.activeClient = nil
	cc.activeProfile = ""
	cc.activeIndex = -1
	cc.logger.Info("[CLUSTER] Cluster client closed for group '%s'", cc.groupName)
}

// GetCandidateNames returns the names of all failover candidates in order.
func (cc *ClusterClient) GetCandidateNames() []string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	names := make([]string, len(cc.candidates))
	for i, c := range cc.candidates {
		names[i] = c.Name
	}
	return names
}