package plugins

import (
	"sort"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/internal/ui/plugins/commandrunner"
	"github.com/devnullvoid/pvetui/internal/ui/plugins/communityscripts"
	"github.com/devnullvoid/pvetui/internal/ui/plugins/guestlist"
)

type factory func() components.Plugin

var registry = map[string]factory{
	commandrunner.PluginID:    func() components.Plugin { return commandrunner.New() },
	communityscripts.PluginID: func() components.Plugin { return communityscripts.New() },
	guestlist.PluginID:        func() components.Plugin { return guestlist.New() },
}

var defaultEnabled = []string{}

// EnabledFromConfig resolves the effective plugin set for the provided configuration.
func EnabledFromConfig(cfg *config.Config) ([]components.Plugin, []string) {
	desired := cfg.Plugins.Enabled
	if desired == nil {
		desired = defaultEnabled
	}

	return resolve(desired)
}

func resolve(ids []string) ([]components.Plugin, []string) {
	seen := make(map[string]struct{})
	plugins := make([]components.Plugin, 0, len(ids))
	var missing []string

	for _, id := range ids {
		if id == "" {
			continue
		}

		if _, exists := seen[id]; exists {
			continue
		}

		factory, ok := registry[id]
		if !ok {
			missing = append(missing, id)
			continue
		}

		plugins = append(plugins, factory())
		seen[id] = struct{}{}
	}

	return plugins, missing
}

// AvailableIDs returns the list of registered plugin identifiers.
func AvailableIDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}

	return ids
}

// Info describes a plugin's identity and user-facing metadata.
type Info struct {
	ID          string
	Name        string
	Description string
}

// AvailableMetadata returns metadata for all registered plugins sorted by name.
func AvailableMetadata() []Info {
	infos := make([]Info, 0, len(registry))
	for id, factory := range registry {
		instance := factory()
		infos = append(infos, Info{
			ID:          id,
			Name:        instance.Name(),
			Description: instance.Description(),
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos
}
