package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// StartVMStatusRefresh begins periodic monitoring of VM status and updates UI colors
// to indicate VM status (white for running, gray for stopped)
func StartVMStatusRefresh(app *tview.Application, client *api.Client, vmList *tview.List, vms []api.VM) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Process VMs in small batches (max 3 per batch) to reduce UI pressure
		processBatch := func(start, end int) {
			// Results channel collects status updates to apply in a single UI update
			results := make(chan struct {
				idx     int
				isActive bool
				name    string
			}, end-start)

			// Process each VM in batch
			for i := start; i < end; i++ {
				i := i // Capture for goroutine
				if i >= len(vms) {
					break
				}
				vm := vms[i]

				// Process this VM's status
				go func() {
					// Create a result that preserves current state by default
					result := struct {
						idx     int
						isActive bool 
						name    string
					}{
						idx:     i,
						// Default to active (white) unless proven inactive
						isActive: true,
						name:    vm.Name,
					}

					// Create timeout context for this API call
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()

					// Run status fetch with timeout
					doneCh := make(chan struct{})
					go func() {
						defer close(doneCh)
						if status, err := client.GetVmStatus(vm); err == nil {
							// Check status field if available
							if st, ok := status["status"].(string); ok {
								// Any of these indicate the VM is inactive
								inactiveStates := []string{"stopped", "stop", "shutdown"}
								for _, inactive := range inactiveStates {
									if strings.EqualFold(st, inactive) {
										result.isActive = false
										break
									}
								}
							}
						}
					}()

					// Wait for completion or timeout
					select {
					case <-doneCh:
						// Successfully got status
					case <-ctx.Done():
						// Timed out, use default (active)
					}

					// Send result
					results <- result
				}()
			}

			// Collect results and apply in a single UI update
			var updates []struct {
				idx     int
				isActive bool
				name    string
			}

			// Wait for results with a timeout
			timeout := time.After(2500 * time.Millisecond)
			for i := 0; i < end-start && i < len(vms); i++ {
				select {
				case result := <-results:
					updates = append(updates, result)
				case <-timeout:
					// Stop waiting if timeout reached
					goto applyUpdates
				}
			}

		applyUpdates:
			// Apply all updates in a single UI refresh to reduce jitter
			if len(updates) > 0 {
				app.QueueUpdateDraw(func() {
					for _, update := range updates {
						text := fmt.Sprintf("%d - %s", vms[update.idx].ID, update.name)
						if update.isActive {
							vmList.SetItemText(update.idx, text, "")
						} else {
							vmList.SetItemText(update.idx, "[gray]"+text, "")
						}
					}
				})
			}
		}

		for range ticker.C {
			// Process VMs in small batches with a delay between batches
			batchSize := 3
			for i := 0; i < len(vms); i += batchSize {
				end := i + batchSize
				if end > len(vms) {
					end = len(vms)
				}
				processBatch(i, end)
				
				// Small delay between batches to reduce resource contention
				if i+batchSize < len(vms) {
					time.Sleep(500 * time.Millisecond)
				}
			}
		}
	}()
}
