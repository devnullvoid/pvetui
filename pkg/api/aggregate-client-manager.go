// Package api provides Proxmox API client functionality.
package api

import (
	"context"
	"fmt"
	"sync"

	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

// ProfileClient wraps an API client with its profile information.
// This represents a single Proxmox connection within an aggregate cluster.
type ProfileClient struct {
	Client      *Client
	ProfileName string // The profile name from config
	Status      ProfileConnectionStatus
	LastErr     error
	mu          sync.RWMutex
}

// ProfileConnectionStatus represents the connection state of a profile.
type ProfileConnectionStatus int

const (
	// ProfileStatusUnknown indicates the profile connection status is unknown.
	ProfileStatusUnknown ProfileConnectionStatus = iota
	// ProfileStatusConnected indicates the profile is successfully connected.
	ProfileStatusConnected
	// ProfileStatusDisconnected indicates the profile is disconnected.
	ProfileStatusDisconnected
	// ProfileStatusError indicates the profile connection has an error.
	ProfileStatusError
)

// String returns the string representation of the connection status.
func (s ProfileConnectionStatus) String() string {
	switch s {
	case ProfileStatusConnected:
		return "connected"
	case ProfileStatusDisconnected:
		return "disconnected"
	case ProfileStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// SetStatus safely updates the profile's connection status.
func (pc *ProfileClient) SetStatus(status ProfileConnectionStatus, err error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.Status = status
	pc.LastErr = err
}

// GetStatus safely retrieves the profile's connection status.
func (pc *ProfileClient) GetStatus() (ProfileConnectionStatus, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.Status, pc.LastErr
}

// AggregateClientManager manages multiple Proxmox API clients for aggregate cluster mode.
// It provides concurrent access to multiple profiles and aggregates their data.
type AggregateClientManager struct {
	aggregateName string                    // Name of the aggregate group
	clients       map[string]*ProfileClient // keyed by profile name
	logger        interfaces.Logger
	cache         interfaces.Cache
	mu            sync.RWMutex
}

// NewAggregateClientManager creates a new aggregate client manager.
func NewAggregateClientManager(
	aggregateName string,
	logger interfaces.Logger,
	cache interfaces.Cache,
) *AggregateClientManager {
	return &AggregateClientManager{
		aggregateName: aggregateName,
		clients:       make(map[string]*ProfileClient),
		logger:        logger,
		cache:         cache,
	}
}

// ProfileEntry represents a profile to be added to the aggregate manager.
// This is a simple struct to pass profile information without importing config package.
type ProfileEntry struct {
	Name   string
	Config interfaces.Config
}

// Initialize creates and connects clients for all profiles in the aggregate group.
// Returns an error only if ALL connections fail; partial failures are logged.
func (m *AggregateClientManager) Initialize(ctx context.Context, profiles []ProfileEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing clients
	m.clients = make(map[string]*ProfileClient)

	if len(profiles) == 0 {
		return fmt.Errorf("no profiles provided for aggregate group '%s'", m.aggregateName)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(profiles))
	clientChan := make(chan *ProfileClient, len(profiles))

	// Connect to all profiles concurrently
	for _, entry := range profiles {
		wg.Add(1)
		go func(pe ProfileEntry) {
			defer wg.Done()

			pc := &ProfileClient{
				ProfileName: pe.Name,
				Status:      ProfileStatusUnknown,
			}

			// Create cache key prefix for this profile
			// Using simple key prefixing instead of namespace
			cacheKeyPrefix := fmt.Sprintf("aggregate:%s:profile:%s:",
				m.aggregateName, pe.Name)

			// Create client with prefixed cache keys
			// For now, we'll just use the shared cache with prefixed keys
			client, err := NewClient(pe.Config,
				WithLogger(m.logger),
				WithCache(m.cache),
			)

			_ = cacheKeyPrefix // Will be used when we implement cache key prefixing

			if err != nil {
				pc.SetStatus(ProfileStatusError, err)
				m.logger.Error("Failed to connect to profile %s: %v", pe.Name, err)
				errChan <- fmt.Errorf("profile %s: %w", pe.Name, err)
			} else {
				pc.Client = client
				pc.SetStatus(ProfileStatusConnected, nil)
				m.logger.Debug("Successfully connected to profile %s", pe.Name)
			}

			clientChan <- pc
		}(entry)
	}

	// Wait for all connections to complete
	wg.Wait()
	close(errChan)
	close(clientChan)

	// Collect all clients (including failed ones for status tracking)
	for pc := range clientChan {
		m.clients[pc.ProfileName] = pc
	}

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// Check if ALL connections failed
	connectedCount := 0
	for _, pc := range m.clients {
		if status, _ := pc.GetStatus(); status == ProfileStatusConnected {
			connectedCount++
		}
	}

	if connectedCount == 0 {
		return fmt.Errorf("failed to connect to any profiles in aggregate '%s': %v",
			m.aggregateName, errors)
	}

	m.logger.Info("Aggregate client manager '%s' initialized: %d/%d profiles connected",
		m.aggregateName, connectedCount, len(profiles))

	return nil
}

// GetAggregateName returns the name of this aggregate group.
func (m *AggregateClientManager) GetAggregateName() string {
	return m.aggregateName
}

// GetClient returns the client for a specific profile by name.
func (m *AggregateClientManager) GetClient(profileName string) (*ProfileClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, exists := m.clients[profileName]
	return client, exists
}

// GetConnectedClients returns all currently connected profile clients.
func (m *AggregateClientManager) GetConnectedClients() []*ProfileClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connected := make([]*ProfileClient, 0, len(m.clients))
	for _, pc := range m.clients {
		if status, _ := pc.GetStatus(); status == ProfileStatusConnected {
			connected = append(connected, pc)
		}
	}
	return connected
}

// GetAllClients returns all clients regardless of status.
func (m *AggregateClientManager) GetAllClients() []*ProfileClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]*ProfileClient, 0, len(m.clients))
	for _, pc := range m.clients {
		all = append(all, pc)
	}
	return all
}

// Close disconnects all clients and cleans up resources.
func (m *AggregateClientManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, pc := range m.clients {
		pc.SetStatus(ProfileStatusDisconnected, nil)
		m.logger.Debug("Disconnected from profile %s", name)
	}

	m.clients = make(map[string]*ProfileClient)
}

// ConnectionSummary represents the connection status of the aggregate.
type ConnectionSummary struct {
	AggregateName  string
	TotalProfiles  int
	ConnectedCount int
	ErrorCount     int
	ProfileStatus  map[string]string // profile name -> status description
}

// GetConnectionSummary returns a summary of connection statuses.
func (m *AggregateClientManager) GetConnectionSummary() ConnectionSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := ConnectionSummary{
		AggregateName: m.aggregateName,
		TotalProfiles: len(m.clients),
		ProfileStatus: make(map[string]string),
	}

	for name, pc := range m.clients {
		status, err := pc.GetStatus()
		if status == ProfileStatusConnected {
			summary.ConnectedCount++
			summary.ProfileStatus[name] = "connected"
		} else {
			summary.ErrorCount++
			if err != nil {
				summary.ProfileStatus[name] = fmt.Sprintf("error: %v", err)
			} else {
				summary.ProfileStatus[name] = status.String()
			}
		}
	}

	return summary
}
