package plugins

import (
	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/internal/ui/plugins/communityscripts"
)

type factory func() components.Plugin

var registry = map[string]factory{
	communityscripts.PluginID: func() components.Plugin { return communityscripts.New() },
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
