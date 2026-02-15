package components

import (
	"fmt"

	"github.com/devnullvoid/pvetui/pkg/api"
)

func guestSelectionKey(vm *api.VM) string {
	if vm == nil {
		return ""
	}

	return fmt.Sprintf("%s|%s|%d", vm.SourceProfile, vm.Node, vm.ID)
}

func (a *App) toggleGuestSelection(vm *api.VM) bool {
	if vm == nil {
		return false
	}

	if a.guestSelections == nil {
		a.guestSelections = make(map[string]struct{})
	}

	key := guestSelectionKey(vm)
	if key == "" {
		return false
	}

	if _, exists := a.guestSelections[key]; exists {
		delete(a.guestSelections, key)
		a.updateGuestSelectionIndicators()

		return false
	}

	a.guestSelections[key] = struct{}{}
	a.updateGuestSelectionIndicators()

	return true
}

func (a *App) guestSelectionCount() int {
	if a == nil {
		return 0
	}

	return len(a.guestSelections)
}

func (a *App) isGuestSelected(vm *api.VM) bool {
	if a == nil || vm == nil {
		return false
	}

	key := guestSelectionKey(vm)
	if key == "" {
		return false
	}

	_, ok := a.guestSelections[key]

	return ok
}

func (a *App) reconcileGuestSelection(vms []*api.VM) {
	if a == nil || len(a.guestSelections) == 0 {
		return
	}

	existing := make(map[string]struct{}, len(vms))
	for _, vm := range vms {
		if vm == nil {
			continue
		}
		existing[guestSelectionKey(vm)] = struct{}{}
	}

	for key := range a.guestSelections {
		if _, ok := existing[key]; !ok {
			delete(a.guestSelections, key)
		}
	}

	a.updateGuestSelectionIndicators()
}

func (a *App) updateGuestSelectionIndicators() {
	if a == nil {
		return
	}

	if a.footer != nil {
		a.footer.UpdateSelectedGuestsCount(a.guestSelectionCount())
	}
}
