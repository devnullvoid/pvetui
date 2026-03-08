package components

import (
	"strings"

	"github.com/devnullvoid/pvetui/pkg/api"
)

func isTemplateGuest(vm *api.VM) bool {
	return vm != nil && vm.Template
}

func guestStatusLabel(vm *api.VM) string {
	if isTemplateGuest(vm) {
		return "Template"
	}
	if vm == nil || vm.Status == "" {
		return ""
	}

	return strings.ToUpper(vm.Status[:1]) + vm.Status[1:]
}

func guestTypeLabel(vm *api.VM) string {
	if vm == nil {
		return ""
	}

	guestType := strings.ToUpper(vm.Type)
	if isTemplateGuest(vm) {
		return guestType + " Template"
	}

	return guestType
}

func canStartGuest(vm *api.VM) bool {
	return vm != nil && !vm.Template && vm.Status == api.VMStatusStopped
}
