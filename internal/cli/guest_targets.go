package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/spf13/cobra"
)

// parseGuestTargetID returns zero for names; numeric targets always mean IDs.
func parseGuestTargetID(target string) (int, error) {
	if strings.TrimSpace(target) == "" {
		return 0, fmt.Errorf("guest ID or name is required")
	}
	numeric := strings.TrimPrefix(strings.TrimPrefix(target, "+"), "-")
	if numeric == "" {
		return 0, fmt.Errorf("invalid guest target %q", target)
	}
	for _, ch := range numeric {
		if ch < '0' || ch > '9' {
			return 0, nil
		}
	}
	return parseVMID(target)
}

func (s *cliSession) guestInventory(ctx context.Context, node string) ([]*api.VM, error) {
	if node != "" {
		client, err := s.clientForGuestNode(ctx, node)
		if err != nil {
			return nil, err
		}
		guests, err := client.ListNodeGuestsContext(ctx, node)
		for _, vm := range guests {
			s.attachGuestProfile(client, vm)
		}
		return guests, err
	}
	if s.group == nil {
		return s.single.ListClusterGuests(ctx)
	}
	if len(s.group.GetConnectedClients()) != len(s.group.GetAllClients()) {
		return nil, fmt.Errorf("guest name lookup requires all group profiles to be available; narrow the target with --profile or --node")
	}
	results := s.group.ExecuteOnAllProfiles(ctx, func(profile string, client *api.Client) (interface{}, error) {
		vms, err := client.ListClusterGuests(ctx)
		for _, vm := range vms {
			vm.SourceProfile = profile
		}
		return vms, err
	})
	var guests []*api.VM
	for _, result := range results {
		if !result.Success {
			return nil, fmt.Errorf("incomplete guest inventory: %w; narrow the target with --profile or --node", result.Error)
		}
		vms, ok := result.Data.([]*api.VM)
		if !ok {
			return nil, fmt.Errorf("invalid guest inventory for profile %q", result.ProfileName)
		}
		guests = append(guests, vms...)
	}
	sort.Slice(guests, func(i, j int) bool {
		if guests[i].SourceProfile != guests[j].SourceProfile {
			return guests[i].SourceProfile < guests[j].SourceProfile
		}
		return guests[i].ID < guests[j].ID
	})
	return guests, nil
}

func (s *cliSession) clientForGuestNode(ctx context.Context, node string) (*api.Client, error) {
	if s.group == nil {
		return s.single, nil
	}
	if len(s.group.GetConnectedClients()) != len(s.group.GetAllClients()) {
		return nil, fmt.Errorf("node ownership requires all group profiles to be available; use --profile to narrow the target")
	}
	results := s.group.ExecuteOnAllProfiles(ctx, func(_ string, client *api.Client) (interface{}, error) {
		return client.ListNodesContext(ctx)
	})
	var profiles []string
	for _, result := range results {
		if !result.Success {
			return nil, fmt.Errorf("cannot resolve node ownership: %w; use --profile to narrow the target", result.Error)
		}
		nodes, ok := result.Data.([]api.Node)
		if !ok {
			return nil, fmt.Errorf("invalid node inventory for profile %q", result.ProfileName)
		}
		for _, n := range nodes {
			if n.Name == node {
				profiles = append(profiles, result.ProfileName)
				break
			}
		}
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("node %q not found", node)
	}
	if len(profiles) > 1 {
		sort.Strings(profiles)
		return nil, fmt.Errorf("node %q is ambiguous across profiles %s; use --profile", node, strings.Join(profiles, ", "))
	}
	pc, ok := s.group.GetClient(profiles[0])
	if !ok || pc.Client == nil {
		return nil, fmt.Errorf("profile %q unavailable", profiles[0])
	}
	return pc.Client, nil
}

func (s *cliSession) attachGuestProfile(client *api.Client, vm *api.VM) {
	if s.group == nil {
		return
	}
	for _, pc := range s.group.GetConnectedClients() {
		if pc.Client == client {
			vm.SourceProfile = pc.ProfileName
			return
		}
	}
}

func matchGuestName(guests []*api.VM, name, guestType string) (*api.VM, error) {
	var matches []*api.VM
	for _, vm := range guests {
		if vm != nil && vm.Name == name && (guestType == "" || vm.Type == guestType) {
			matches = append(matches, vm)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("guest named %q not found", name)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	var message strings.Builder
	fmt.Fprintf(&message, "guest name %q is ambiguous:\n", name)
	for _, vm := range matches {
		fmt.Fprintf(&message, "  ID %d (%s) on %s, profile %s\n", vm.ID, vm.Type, vm.Node, vm.SourceProfile)
	}
	message.WriteString("Use a guest ID, --node, or --profile to select one.")
	return nil, fmt.Errorf("%s", message.String())
}

func resolveGuestTarget(ctx context.Context, cmd *cobra.Command, s *cliSession, target string, enrich bool) (*api.VM, error) {
	id, err := parseGuestTargetID(target)
	if err != nil {
		return nil, err
	}
	node, _ := cmd.Flags().GetString("node")
	guestType := ""
	if cmd.Flags().Changed("type") {
		guestType, _ = cmd.Flags().GetString("type")
		if guestType != api.VMTypeQemu && guestType != api.VMTypeLXC {
			return nil, fmt.Errorf("invalid guest type %q; expected qemu or lxc", guestType)
		}
	}
	if id > 0 {
		if cmd.Name() == "exec" {
			return findVMForCommand(ctx, cmd, s, id, false)
		}
		vm, err := findVMForCommand(ctx, cmd, s, id, enrich)
		if err != nil || node == "" || cmd.Name() != "shell" {
			return vm, err
		}
		client, err := s.clientForVM(vm)
		if err != nil {
			return nil, err
		}
		fresh, err := client.RefreshVMData(vm, nil)
		if err != nil {
			return nil, err
		}
		fresh.SourceProfile = vm.SourceProfile
		return fresh, nil
	}
	guests, err := s.guestInventory(ctx, node)
	if err != nil {
		return nil, err
	}
	vm, err := matchGuestName(guests, target, guestType)
	if err != nil || !enrich {
		return vm, err
	}
	client, err := s.clientForVM(vm)
	if err != nil {
		return nil, err
	}
	fresh, err := client.RefreshVMData(vm, nil)
	if err != nil {
		return nil, err
	}
	fresh.SourceProfile = vm.SourceProfile
	return fresh, nil
}

func guestCompletions(guests []*api.VM, prefix, guestType string) []string {
	var out []string
	names := make(map[string][]string)
	ids := make(map[string][]string)
	for _, vm := range guests {
		if vm == nil || (guestType != "" && vm.Type != guestType) {
			continue
		}
		id := strconv.Itoa(vm.ID)
		location := fmt.Sprintf("%s on %s", vm.Type, vm.Node)
		if vm.SourceProfile != "" {
			location += " [" + vm.SourceProfile + "]"
		}
		ids[id] = append(ids[id], fmt.Sprintf("%s (%s)", vm.Name, location))
		if nameID, err := parseGuestTargetID(vm.Name); err == nil && nameID == 0 {
			names[vm.Name] = append(names[vm.Name], fmt.Sprintf("ID %s (%s)", id, location))
		}
	}
	for id, descriptions := range ids {
		if strings.HasPrefix(id, prefix) {
			out = append(out, id+"\t"+strings.Join(descriptions, "; "))
		}
	}
	for name, descriptions := range names {
		if strings.HasPrefix(name, prefix) {
			out = append(out, name+"\t"+strings.Join(descriptions, "; "))
		}
	}
	sort.Strings(out)
	return out
}

// completionInventory caches suggestions only; target resolution must stay fresh.
func (s *cliSession) completionInventory(ctx context.Context, node string) ([]*api.VM, error) {
	if s.completionCache == nil || s.cfg == nil {
		return s.guestInventory(ctx, node)
	}
	identity := [][]string{{node, s.cfg.ActiveProfile, s.cfg.Addr, s.cfg.User, s.cfg.Realm, s.cfg.TokenID}}
	if s.group != nil {
		for _, pc := range s.group.GetAllClients() {
			p := s.cfg.Profiles[pc.ProfileName]
			identity = append(identity, []string{pc.ProfileName, p.Addr, p.ApiPath, p.User, p.Realm, p.TokenID})
		}
		sort.Slice(identity[1:], func(i, j int) bool { return identity[i+1][0] < identity[j+1][0] })
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return s.guestInventory(ctx, node)
	}
	key := fmt.Sprintf("cli:guest-completion:v1:%x", sha256.Sum256(encoded))
	var guests []*api.VM
	if found, cacheErr := s.completionCache.Get(key, &guests); cacheErr == nil && found {
		return guests, nil
	}
	guests, err = s.guestInventory(ctx, node)
	if err == nil && ctx.Err() == nil {
		_ = s.completionCache.Set(key, guests, time.Minute)
	}
	return guests, err
}

func completeGuestTargets(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	return completeGuestTargetsWithin(cmd, args, prefix, api.DefaultAPITimeout)
}

func completeGuestTargetsWithin(cmd *cobra.Command, args []string, prefix string, timeout time.Duration) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		if cmd.Name() != "migrate" || len(args) != 1 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	// Bootstrap and legacy node resolution can perform requests without caller
	// context. Bound the whole completion as well as the inventory requests.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	type result struct {
		values []string
		err    error
	}
	ready := make(chan result, 1)
	go func() {
		if len(args) == 1 {
			values, _ := completeNodeNames(cmd, nil, prefix)
			ready <- result{values: values}
			return
		}
		session, err := initCLISession(cmd)
		if err != nil || session == nil {
			ready <- result{err: fmt.Errorf("completion session unavailable")}
			return
		}
		node, _ := cmd.Flags().GetString("node")
		vms, err := session.completionInventory(ctx, node)
		guestType := ""
		if cmd.Flags().Changed("type") {
			guestType, _ = cmd.Flags().GetString("type")
		}
		if cmd.Flags().Lookup("guest") != nil {
			guestType = api.VMTypeLXC
		}
		ready <- result{values: guestCompletions(vms, prefix, guestType), err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, cobra.ShellCompDirectiveNoFileComp
	case r := <-ready:
		if r.err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return r.values, cobra.ShellCompDirectiveNoFileComp
	}
}
