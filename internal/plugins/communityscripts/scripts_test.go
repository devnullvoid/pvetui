package communityscripts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchScripts(t *testing.T) {
	scripts := []Script{
		{Name: "Nextcloud", Slug: "nextcloud", Description: "Cloud storage"},
		{Name: "Home Assistant", Slug: "homeassistant", Description: "Automation hub"},
		{Name: "Docker", Slug: "docker", Description: "Container runtime"},
	}

	matches := SearchScripts(scripts, "cloud")
	require.Len(t, matches, 1)
	require.Equal(t, "nextcloud", matches[0].Slug)

	matches = SearchScripts(scripts, "HOME")
	require.Len(t, matches, 1)
	require.Equal(t, "homeassistant", matches[0].Slug)

	matches = SearchScripts(scripts, "")
	require.Len(t, matches, 3)
}

func TestFindScript(t *testing.T) {
	scripts := []Script{
		{Name: "Nextcloud", Slug: "nextcloud"},
		{Name: "Nextcloud Backup", Slug: "nextcloud-backup"},
		{Name: "Home Assistant", Slug: "homeassistant"},
	}

	script, err := FindScript(scripts, "Home Assistant")
	require.NoError(t, err)
	require.Equal(t, "homeassistant", script.Slug)

	script, err = FindScript(scripts, "nextcloud")
	require.NoError(t, err)
	require.Equal(t, "Nextcloud", script.Name)

	_, err = FindScript(scripts, "next")
	require.ErrorContains(t, err, "ambiguous")

	_, err = FindScript(scripts, "missing")
	require.ErrorContains(t, err, "not found")
}
