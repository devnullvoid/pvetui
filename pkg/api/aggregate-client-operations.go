// Package api provides Proxmox API client functionality.
package api

import (
	"context"
	"fmt"
	"sync"

	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

// ProfileResult represents the result of an operation on a single profile.
type ProfileResult struct {
	ProfileName string
	Success     bool
	Error       error
	Data        interface{} // Optional data returned from the operation
}

// ProfileOperation is a function that performs an operation on a single client.
// It receives the profile name, client, and returns optional data and an error.
type ProfileOperation func(profileName string, client *Client) (interface{}, error)

// ExecuteOnAllProfiles executes an operation concurrently on all connected profiles.
// Returns a slice of ProfileResult, one for each profile attempted.
// This method does not fail if some profiles fail; it collects all results.
func (m *AggregateClientManager) ExecuteOnAllProfiles(
	ctx context.Context,
	operation ProfileOperation,
) []ProfileResult {
	m.mu.RLock()
	clients := m.GetConnectedClients()
	m.mu.RUnlock()

	results := make([]ProfileResult, 0, len(clients))
	resultsMutex := sync.Mutex{}

	var wg sync.WaitGroup
	for _, pc := range clients {
		wg.Add(1)
		go func(profileClient *ProfileClient) {
			defer wg.Done()

			result := ProfileResult{
				ProfileName: profileClient.ProfileName,
				Success:     false,
			}

			// Check context cancellation before executing
			select {
			case <-ctx.Done():
				result.Error = ctx.Err()
				resultsMutex.Lock()
				results = append(results, result)
				resultsMutex.Unlock()
				return
			default:
			}

			// Execute the operation
			data, err := operation(profileClient.ProfileName, profileClient.Client)
			if err != nil {
				result.Error = fmt.Errorf("profile %s: %w", profileClient.ProfileName, err)
				m.logger.Error("Operation failed for profile %s: %v", profileClient.ProfileName, err)
			} else {
				result.Success = true
				result.Data = data
				m.logger.Debug("Operation succeeded for profile %s", profileClient.ProfileName)
			}

			resultsMutex.Lock()
			results = append(results, result)
			resultsMutex.Unlock()
		}(pc)
	}

	wg.Wait()
	return results
}

// ExecuteOnProfile executes an operation on a specific profile by name.
// Returns the operation data and error.
func (m *AggregateClientManager) ExecuteOnProfile(
	ctx context.Context,
	profileName string,
	operation ProfileOperation,
) (interface{}, error) {
	pc, exists := m.GetClient(profileName)
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found in aggregate '%s'", profileName, m.aggregateName)
	}

	status, _ := pc.GetStatus()
	if status != ProfileStatusConnected {
		return nil, fmt.Errorf("profile '%s' is not connected (status: %s)", profileName, status.String())
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return operation(profileName, pc.Client)
}

// RefreshProfileConnection attempts to reconnect a failed profile.
// This is useful for recovering from transient network errors.
func (m *AggregateClientManager) RefreshProfileConnection(
	ctx context.Context,
	profileName string,
	profileConfig interfaces.Config,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pc, exists := m.clients[profileName]
	if !exists {
		return fmt.Errorf("profile '%s' not found in aggregate '%s'", profileName, m.aggregateName)
	}

	// Set status to unknown while reconnecting
	pc.SetStatus(ProfileStatusUnknown, nil)
	m.logger.Info("Attempting to reconnect profile %s", profileName)

	// Check context cancellation
	select {
	case <-ctx.Done():
		pc.SetStatus(ProfileStatusError, ctx.Err())
		return ctx.Err()
	default:
	}

	// Create new client
	client, err := NewClient(profileConfig,
		WithLogger(m.logger),
		WithCache(m.cache),
	)

	if err != nil {
		pc.SetStatus(ProfileStatusError, err)
		m.logger.Error("Failed to reconnect to profile %s: %v", profileName, err)
		return fmt.Errorf("failed to reconnect profile %s: %w", profileName, err)
	}

	// Update the profile client
	pc.Client = client
	pc.SetStatus(ProfileStatusConnected, nil)
	m.logger.Info("Successfully reconnected to profile %s", profileName)

	return nil
}

// RefreshAllFailedProfiles attempts to reconnect all profiles with error status.
// Returns a map of profile names to their reconnection results (nil on success, error on failure).
func (m *AggregateClientManager) RefreshAllFailedProfiles(
	ctx context.Context,
	profileConfigs map[string]interfaces.Config,
) map[string]error {
	m.mu.RLock()
	failedProfiles := make([]*ProfileClient, 0)
	for _, pc := range m.clients {
		status, _ := pc.GetStatus()
		if status == ProfileStatusError || status == ProfileStatusDisconnected {
			failedProfiles = append(failedProfiles, pc)
		}
	}
	m.mu.RUnlock()

	results := make(map[string]error)
	resultsMutex := sync.Mutex{}

	var wg sync.WaitGroup
	for _, pc := range failedProfiles {
		wg.Add(1)
		go func(profileClient *ProfileClient) {
			defer wg.Done()

			profileConfig, exists := profileConfigs[profileClient.ProfileName]
			if !exists {
				err := fmt.Errorf("config not found for profile %s", profileClient.ProfileName)
				resultsMutex.Lock()
				results[profileClient.ProfileName] = err
				resultsMutex.Unlock()
				return
			}

			err := m.RefreshProfileConnection(ctx, profileClient.ProfileName, profileConfig)
			resultsMutex.Lock()
			results[profileClient.ProfileName] = err
			resultsMutex.Unlock()
		}(pc)
	}

	wg.Wait()
	return results
}

// GetAggregatedData is a helper for executing an operation and aggregating results.
// It executes the operation on all connected profiles and returns combined results.
// The aggregateFunc receives all successful results and should combine them.
func (m *AggregateClientManager) GetAggregatedData(
	ctx context.Context,
	operation ProfileOperation,
	aggregateFunc func(results []ProfileResult) (interface{}, error),
) (interface{}, error) {
	results := m.ExecuteOnAllProfiles(ctx, operation)

	// Filter successful results
	successfulResults := make([]ProfileResult, 0, len(results))
	for _, result := range results {
		if result.Success {
			successfulResults = append(successfulResults, result)
		}
	}

	if len(successfulResults) == 0 {
		return nil, fmt.Errorf("no profiles returned successful results")
	}

	return aggregateFunc(successfulResults)
}
