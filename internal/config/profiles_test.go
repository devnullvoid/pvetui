package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGroups(t *testing.T) {
	tests := []struct {
		name           string
		profiles       map[string]ProfileConfig
		expectedGroups map[string][]string
	}{
		{
			name: "single group",
			profiles: map[string]ProfileConfig{
				"p1": {Groups: []string{"group1"}},
				"p2": {Groups: []string{"group1"}},
			},
			expectedGroups: map[string][]string{
				"group1": {"p1", "p2"},
			},
		},
		{
			name: "multiple groups",
			profiles: map[string]ProfileConfig{
				"p1": {Groups: []string{"group1", "group2"}},
				"p2": {Groups: []string{"group1"}},
				"p3": {Groups: []string{"group2"}},
			},
			expectedGroups: map[string][]string{
				"group1": {"p1", "p2"},
				"group2": {"p1", "p3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Profiles: tt.profiles}
			groups := cfg.GetGroups()
			assert.Equal(t, tt.expectedGroups, groups)
		})
	}
}

func TestGetProfilesInGroup(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]ProfileConfig{
			"p1": {Groups: []string{"group1", "group2"}},
			"p2": {Groups: []string{"group1"}},
			"p3": {Groups: []string{"group2"}},
			"p4": {Groups: []string{"group1"}},
		},
	}

	// Test Group 1
	pNames1 := cfg.GetProfileNamesInGroup("group1")
	assert.ElementsMatch(t, []string{"p1", "p2", "p4"}, pNames1)

	// Test Group 2
	pNames2 := cfg.GetProfileNamesInGroup("group2")
	assert.ElementsMatch(t, []string{"p1", "p3"}, pNames2)

	// Test Non-existent Group
	pNames3 := cfg.GetProfileNamesInGroup("group3")
	assert.Empty(t, pNames3)
}

func TestValidateGroupsRejectsStaleGroupSettings(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]ProfileConfig{
			"p1": {Groups: []string{"group1"}},
		},
		GroupSettings: map[string]GroupSettingsConfig{
			"deleted-group": {Mode: GroupModeCluster},
		},
	}

	err := cfg.ValidateGroups()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "group_settings 'deleted-group' does not match any group")
}

func TestValidateGroupsAcceptsClusterModeSettingForExistingGroup(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]ProfileConfig{
			"p1": {Groups: []string{"group1"}},
		},
		GroupSettings: map[string]GroupSettingsConfig{
			"group1": {Mode: GroupModeCluster},
		},
	}

	err := cfg.ValidateGroups()
	assert.NoError(t, err)
	assert.True(t, cfg.IsClusterGroup("group1"))
}
