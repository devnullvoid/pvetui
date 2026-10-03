package components

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/devnullvoid/pvetui/internal/config"
)

func TestApplicationSettingsPreferences(t *testing.T) {
	for _, action := range []string{"save", "cancel", "invalid interval", "save failure"} {
		t.Run(action, func(t *testing.T) {
			cfg := config.NewConfig()
			cfg.Debug = false
			app := &App{
				Application: tview.NewApplication(), config: *cfg,
				configPath: filepath.Join(t.TempDir(), "config.yml"),
				pages:      tview.NewPages(), header: NewHeader(), footer: NewFooter(),
			}
			if action == "save failure" {
				require.NoError(t, os.WriteFile(app.configPath, []byte("blocked"), 0o600))
				app.configPath = filepath.Join(app.configPath, "config.yml")
			}
			app.showApplicationSettingsDialog()
			_, modal := app.pages.GetFrontPage()
			form := modal.(*centeredCappedModal).content.(*tview.Form)
			require.True(t, form.GetFormItemByLabel("Confirm Quit").(*tview.Checkbox).IsChecked())
			form.GetFormItemByLabel("Confirm Quit").(*tview.Checkbox).SetChecked(false)
			form.GetFormItemByLabel("Quiet Startup").(*tview.Checkbox).SetChecked(true)
			form.GetFormItemByLabel("Startup View").(*tview.DropDown).SetCurrentOption(1)
			form.GetFormItemByLabel("Auto-refresh on Startup").(*tview.Checkbox).SetChecked(true)
			interval := form.GetFormItemByLabel("Refresh Interval (seconds)").(*tview.InputField)
			interval.SetText("30")
			if action == "invalid interval" {
				interval.SetText("4")
			}
			button := "Save"
			if action == "cancel" {
				button = "Cancel"
			}
			form.GetButton(form.GetButtonIndex(button)).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

			if action != "save" {
				require.Equal(t, *cfg, app.config)
				if action == "cancel" {
					require.False(t, app.pages.HasPage("applicationSettings"))
				} else {
					require.True(t, app.pages.HasPage("applicationSettings"))
					require.True(t, app.pages.HasPage("message"))
				}
				return
			}
			require.False(t, app.pages.HasPage("applicationSettings"))
			reloaded := config.NewConfig()
			require.NoError(t, reloaded.MergeWithFile(app.configPath))
			require.False(t, reloaded.ConfirmQuit)
			require.True(t, reloaded.QuietStartup)
			require.Equal(t, "guests", reloaded.StartupPage)
			require.Equal(t, config.AutoRefreshConfig{Enabled: true, Interval: 30}, reloaded.AutoRefresh)
			require.False(t, app.autoRefreshEnabled, "startup preference must not toggle the current session")
			require.Contains(t, app.helpModal.textView.GetText(true), "30s interval")
		})
	}
}

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
