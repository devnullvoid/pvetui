package ansible

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devnullvoid/pvetui/pkg/api"
	"gopkg.in/yaml.v3"
)

const (
	InventoryFormatINI     = "ini"
	InventoryFormatYAML    = "yaml"
	InventoryStyleCompact  = "compact"
	InventoryStyleExpanded = "expanded"
)

// InventoryDefaults defines default SSH users for inventory generation.
type InventoryDefaults struct {
	NodeSSHUser       string
	VMSSHUser         string
	SSHPrivateKeyFile string
	DefaultPassword   string
	Style             string
}

// InventoryHost captures one generated host entry.
type InventoryHost struct {
	Alias      string
	Target     string
	GroupNames []string
	Vars       map[string]string
}

// InventoryResult contains the final inventory text and generated host metadata.
type InventoryResult struct {
	Format string
	Text   string
	Hosts  []InventoryHost
}

// BuildInventory renders an INI-style Ansible inventory from Proxmox nodes and guests.
func BuildInventory(nodes []*api.Node, guests []*api.VM, defaults InventoryDefaults) InventoryResult {
	return BuildInventoryWithFormat(nodes, guests, defaults, InventoryFormatINI)
}

// BuildInventoryWithFormat renders an inventory in the requested format.
func BuildInventoryWithFormat(nodes []*api.Node, guests []*api.VM, defaults InventoryDefaults, format string) InventoryResult {
	format = NormalizeInventoryFormat(format)
	style := NormalizeInventoryStyle(defaults.Style)

	hosts := make([]InventoryHost, 0, len(nodes)+len(guests))
	groups := make(map[string][]InventoryHost)

	pushHost := func(host InventoryHost) {
		hosts = append(hosts, host)
		for _, group := range host.GroupNames {
			groups[group] = append(groups[group], host)
		}
	}

	for _, node := range nodes {
		if node == nil {
			continue
		}

		target := chooseHostTarget(node.IP, node.Name)
		if target == "" {
			continue
		}

		alias := uniqueAlias(hosts, "node_"+sanitizeIdentifier(node.Name))
		vars := map[string]string{
			"ansible_host":  target,
			"ansible_user":  defaults.NodeSSHUser,
			"pvetui_kind":   "node",
			"pvetui_node":   node.Name,
			"pvetui_online": fmt.Sprintf("%t", node.Online),
		}
		if node.SourceProfile != "" {
			vars["pvetui_source_profile"] = node.SourceProfile
		}
		if strings.TrimSpace(defaults.SSHPrivateKeyFile) != "" {
			vars["ansible_ssh_private_key_file"] = strings.TrimSpace(defaults.SSHPrivateKeyFile)
		}
		if strings.TrimSpace(defaults.DefaultPassword) != "" {
			vars["ansible_password"] = strings.TrimSpace(defaults.DefaultPassword)
		}

		groupNames := []string{"proxmox_nodes"}
		if node.Online {
			groupNames = append(groupNames, "running")
		} else {
			groupNames = append(groupNames, "stopped")
		}

		pushHost(InventoryHost{
			Alias:      alias,
			Target:     target,
			GroupNames: groupNames,
			Vars:       vars,
		})
	}

	for _, guest := range guests {
		if guest == nil {
			continue
		}
		if guest.Template {
			continue
		}

		target := chooseHostTarget(guest.IP, guest.Name)
		if target == "" {
			continue
		}

		aliasSeed := fmt.Sprintf("guest_%d_%s", guest.ID, sanitizeIdentifier(guest.Name))
		alias := uniqueAlias(hosts, aliasSeed)

		vars := map[string]string{
			"ansible_host":      target,
			"ansible_user":      defaults.VMSSHUser,
			"pvetui_kind":       "guest",
			"pvetui_guest_id":   fmt.Sprintf("%d", guest.ID),
			"pvetui_guest_type": guest.Type,
			"pvetui_status":     guest.Status,
			"pvetui_node":       guest.Node,
		}
		if guest.SourceProfile != "" {
			vars["pvetui_source_profile"] = guest.SourceProfile
		}
		if strings.TrimSpace(guest.Tags) != "" {
			vars["pvetui_tags"] = guest.Tags
		}
		if strings.TrimSpace(defaults.SSHPrivateKeyFile) != "" {
			vars["ansible_ssh_private_key_file"] = strings.TrimSpace(defaults.SSHPrivateKeyFile)
		}
		if strings.TrimSpace(defaults.DefaultPassword) != "" {
			vars["ansible_password"] = strings.TrimSpace(defaults.DefaultPassword)
		}

		groupNames := []string{"proxmox_guests", "by_node_" + sanitizeIdentifier(guest.Node)}
		switch guest.Type {
		case api.VMTypeQemu:
			groupNames = append(groupNames, "qemu")
		case api.VMTypeLXC:
			groupNames = append(groupNames, "lxc")
		}
		if strings.EqualFold(guest.Status, api.VMStatusRunning) {
			groupNames = append(groupNames, "running")
		} else {
			groupNames = append(groupNames, "stopped")
		}

		pushHost(InventoryHost{
			Alias:      alias,
			Target:     target,
			GroupNames: groupNames,
			Vars:       vars,
		})
	}

	rendered := renderINIInventory(groups, hosts, style)
	if format == InventoryFormatYAML {
		rendered = renderYAMLInventory(groups, hosts, style)
	}

	return InventoryResult{Format: format, Text: rendered, Hosts: hosts}
}

// NormalizeInventoryFormat validates and canonicalizes inventory format.
func NormalizeInventoryFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case InventoryFormatINI:
		return InventoryFormatINI
	case InventoryFormatYAML:
		return InventoryFormatYAML
	default:
		return InventoryFormatYAML
	}
}

// NormalizeInventoryStyle validates and canonicalizes inventory style.
func NormalizeInventoryStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case InventoryStyleExpanded:
		return InventoryStyleExpanded
	case InventoryStyleCompact:
		return InventoryStyleCompact
	default:
		return InventoryStyleCompact
	}
}

func renderINIInventory(groups map[string][]InventoryHost, allHosts []InventoryHost, style string) string {
	orderedGroups := make([]string, 0, len(groups))
	for group := range groups {
		orderedGroups = append(orderedGroups, group)
	}
	sort.Strings(orderedGroups)

	sharedVars := map[string]string{}
	if style == InventoryStyleCompact {
		sharedVars = computeSharedAnsibleVars(allHosts)
	}

	var builder strings.Builder
	builder.WriteString("# Generated by pvetui Ansible plugin\n")
	builder.WriteString("[all:vars]\n")
	builder.WriteString("pvetui_generated=true\n")
	for _, key := range sortedMapKeys(sharedVars) {
		_, _ = fmt.Fprintf(&builder, "%s=%s\n", key, quoteIfNeeded(sharedVars[key]))
	}
	builder.WriteString("\n")

	for _, group := range orderedGroups {
		builder.WriteString("[")
		builder.WriteString(group)
		builder.WriteString("]\n")

		groupHosts := groups[group]
		sort.Slice(groupHosts, func(i, j int) bool {
			return groupHosts[i].Alias < groupHosts[j].Alias
		})

		for _, host := range groupHosts {
			builder.WriteString(host.Alias)
			builder.WriteString(" ")
			builder.WriteString(renderHostVars(host.Vars, sharedVars))
			builder.WriteString("\n")
		}

		builder.WriteString("\n")
	}

	return builder.String()
}

func renderYAMLInventory(groups map[string][]InventoryHost, allHosts []InventoryHost, style string) string {
	type yamlGroup struct {
		Hosts map[string]map[string]string `yaml:"hosts,omitempty"`
	}
	type yamlAll struct {
		Vars     map[string]any       `yaml:"vars,omitempty"`
		Children map[string]yamlGroup `yaml:"children,omitempty"`
	}
	type yamlInventory struct {
		All yamlAll `yaml:"all"`
	}

	orderedGroups := make([]string, 0, len(groups))
	for group := range groups {
		orderedGroups = append(orderedGroups, group)
	}
	sort.Strings(orderedGroups)

	sharedVars := map[string]string{}
	if style == InventoryStyleCompact {
		sharedVars = computeSharedAnsibleVars(allHosts)
	}

	children := make(map[string]yamlGroup, len(orderedGroups))
	for _, group := range orderedGroups {
		groupHosts := groups[group]
		sort.Slice(groupHosts, func(i, j int) bool {
			return groupHosts[i].Alias < groupHosts[j].Alias
		})

		hosts := make(map[string]map[string]string, len(groupHosts))
		for _, host := range groupHosts {
			vars := filterHostVars(host.Vars, sharedVars)
			hosts[host.Alias] = vars
		}

		children[group] = yamlGroup{Hosts: hosts}
	}

	allVars := map[string]any{
		"pvetui_generated": true,
	}
	for _, key := range sortedMapKeys(sharedVars) {
		allVars[key] = sharedVars[key]
	}

	doc := yamlInventory{
		All: yamlAll{
			Vars:     allVars,
			Children: children,
		},
	}

	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Sprintf("# Failed to render YAML inventory: %v\n", err)
	}

	return "# Generated by pvetui Ansible plugin\n" + string(data)
}

func renderHostVars(vars map[string]string, sharedVars map[string]string) string {
	filtered := filterHostVars(vars, sharedVars)

	keys := make([]string, 0, len(filtered))
	for key := range filtered {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, quoteIfNeeded(filtered[key])))
	}

	return strings.Join(parts, " ")
}

func filterHostVars(vars map[string]string, sharedVars map[string]string) map[string]string {
	filtered := make(map[string]string, len(vars))
	for k, v := range vars {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if shared, ok := sharedVars[k]; ok && shared == trimmed {
			continue
		}
		filtered[k] = trimmed
	}
	return filtered
}

func computeSharedAnsibleVars(hosts []InventoryHost) map[string]string {
	if len(hosts) == 0 {
		return map[string]string{}
	}

	shared := make(map[string]string)
	for k, v := range hosts[0].Vars {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" || !isShareableAnsibleVar(k) {
			continue
		}
		shared[k] = trimmed
	}

	for i := 1; i < len(hosts); i++ {
		for key, expected := range shared {
			actual := strings.TrimSpace(hosts[i].Vars[key])
			if actual == "" || actual != expected {
				delete(shared, key)
			}
		}
	}

	return shared
}

func isShareableAnsibleVar(key string) bool {
	if !strings.HasPrefix(key, "ansible_") {
		return false
	}

	return key != "ansible_host"
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func quoteIfNeeded(value string) string {
	if value == "" {
		return `""`
	}

	needsQuotes := strings.ContainsAny(value, " \t;\"'")
	replacer := strings.NewReplacer(`\\`, `\\\\`, `"`, `\\"`)
	escaped := replacer.Replace(value)
	if needsQuotes {
		return `"` + escaped + `"`
	}

	return escaped
}

func chooseHostTarget(candidates ...string) string {
	for _, c := range candidates {
		trimmed := strings.TrimSpace(c)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func sanitizeIdentifier(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "unknown"
	}

	var out strings.Builder
	prevUnderscore := false
	for _, ch := range raw {
		isAlnum := (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
		if isAlnum {
			out.WriteRune(ch)
			prevUnderscore = false
			continue
		}

		if !prevUnderscore {
			out.WriteRune('_')
			prevUnderscore = true
		}
	}

	sanitized := strings.Trim(out.String(), "_")
	if sanitized == "" {
		return "unknown"
	}

	return sanitized
}

func uniqueAlias(existing []InventoryHost, base string) string {
	if base == "" {
		base = "host"
	}

	alias := base
	idx := 1
	for hasAlias(existing, alias) {
		idx++
		alias = fmt.Sprintf("%s_%d", base, idx)
	}

	return alias
}

func hasAlias(hosts []InventoryHost, alias string) bool {
	for _, host := range hosts {
		if host.Alias == alias {
			return true
		}
	}

	return false
}
