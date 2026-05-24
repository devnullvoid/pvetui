package components

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/devnullvoid/pvetui/internal/config"
)

func TestParseStringMapYAML(t *testing.T) {
	t.Parallel()

	values, err := parseStringMapYAML("primary: white\nbackground: '#111111'\n")
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"primary":    "white",
		"background": "#111111",
	}, values)
}

func TestCappedModalDimension(t *testing.T) {
	t.Parallel()

	require.Equal(t, 24, cappedModalDimension(50, 24))
	require.Equal(t, 12, cappedModalDimension(14, 24))
	require.Equal(t, 2, cappedModalDimension(2, 24))
	require.Equal(t, 0, cappedModalDimension(0, 24))
}

func TestSaveConfigPreservingSOPSUsesActiveConfigPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "selected", "config.yml")
	app := &App{
		config: config.Config{
			ShowIcons: true,
			Debug:     true,
		},
		configPath: path,
	}

	require.NoError(t, app.SaveConfigPreservingSOPS())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var saved map[string]any
	require.NoError(t, yaml.Unmarshal(data, &saved))
	require.Equal(t, true, saved["show_icons"])
	require.Equal(t, true, saved["debug"])
}
