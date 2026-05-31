package ansible

import (
	"strings"
	"testing"

	cfgpkg "github.com/devnullvoid/pvetui/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildBootstrapPlaybookIncludesUID(t *testing.T) {
	t.Parallel()

	plugin := &Plugin{}
	playbook, err := plugin.buildBootstrapPlaybook(cfgpkg.AnsibleBootstrapConfig{
		Username:        "ansible",
		UID:             1001,
		Shell:           "/bin/bash",
		CreateHome:      true,
		SudoersFileMode: "0440",
	})

	require.NoError(t, err)
	require.Contains(t, playbook, `bootstrap_uid: "1001"`)
	require.Contains(t, playbook, `uid: "{{ bootstrap_uid if bootstrap_uid | length > 0 else omit }}"`)
}

func TestBuildDirectBootstrapScriptIncludesUID(t *testing.T) {
	t.Parallel()

	script := buildDirectBootstrapScript(cfgpkg.AnsibleBootstrapConfig{
		Username:        "ansible",
		UID:             1001,
		Shell:           "/bin/bash",
		CreateHome:      true,
		SudoersFileMode: "0440",
	}, "", false)

	require.Contains(t, script, "BOOTSTRAP_UID='1001'")
	require.Contains(t, script, `useradd -m -u "$BOOTSTRAP_UID" -s "$BOOTSTRAP_SHELL" "$BOOTSTRAP_USER"`)
	require.Contains(t, script, `usermod -u "$BOOTSTRAP_UID" "$BOOTSTRAP_USER"`)
}

func TestBuildDirectBootstrapScriptOmitsUIDWhenUnset(t *testing.T) {
	t.Parallel()

	script := buildDirectBootstrapScript(cfgpkg.AnsibleBootstrapConfig{
		Username:        "ansible",
		Shell:           "/bin/bash",
		CreateHome:      true,
		SudoersFileMode: "0440",
	}, "", false)

	require.Contains(t, script, "BOOTSTRAP_UID=''")
	require.False(t, strings.Contains(script, "BOOTSTRAP_UID='0'"))
}

func TestResolveAnsibleInventoryUsersDefaultUserOverridesFallbacks(t *testing.T) {
	t.Parallel()

	nodeUser, vmUser := resolveAnsibleInventoryUsers(cfgpkg.AnsiblePluginConfig{
		DefaultUser: "ansible",
	}, "jon", "ubuntu")

	require.Equal(t, "ansible", nodeUser)
	require.Equal(t, "ansible", vmUser)
}

func TestResolveAnsibleInventoryUsersFallsBackToResolvedUsers(t *testing.T) {
	t.Parallel()

	nodeUser, vmUser := resolveAnsibleInventoryUsers(cfgpkg.AnsiblePluginConfig{}, "root", "ubuntu")

	require.Equal(t, "root", nodeUser)
	require.Equal(t, "ubuntu", vmUser)
}
