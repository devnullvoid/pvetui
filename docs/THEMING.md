# Theming Guide

Proxmox TUI supports flexible theming through terminal emulator color schemes and application-level configuration.

## Overview

The application uses semantic color constants that automatically adapt to your terminal emulator's color scheme. This approach provides the best user experience by:

- **Leveraging existing terminal themes**: Use your favorite themes (Dracula, Nord, Solarized, etc.)
- **Maintaining semantic meaning**: Colors always represent the same concepts regardless of theme
- **Minimal configuration**: Works out of the box with most terminal emulators
- **Consistent experience**: Same theme across all terminal applications

## Terminal Emulator Theming

### Supported Themes

The application works well with popular terminal color schemes:

#### Dark Themes
- **Dracula**: Purple-accented dark theme
- **Nord**: Arctic-inspired dark theme
- **Gruvbox**: Retro groove color scheme
- **Catppuccin**: Soothing pastel dark theme
- **Tokyo Night**: Elegant dark theme
- **One Dark**: Atom editor-inspired theme

#### Light Themes
- **Solarized Light**: Carefully designed light palette
- **Gruvbox Light**: Light version of the retro theme
- **Catppuccin Latte**: Light pastel theme

#### High Contrast
- **High Contrast**: Accessibility-focused themes
- **Monokai**: Bold, high-contrast colors

### Setting Up Terminal Themes

#### Alacritty
```yaml
# ~/.config/alacritty/alacritty.yml
colors:
  primary:
    background: '#282a36'
    foreground: '#f8f8f2'
  normal:
    black:   '#000000'
    red:     '#ff5555'
    green:   '#50fa7b'
    yellow:  '#f1fa8c'
    blue:    '#bd93f9'
    magenta: '#ff79c6'
    cyan:    '#8be9fd'
    white:   '#bfbfbf'
  bright:
    black:   '#4d4d4d'
    red:     '#ff6e67'
    green:   '#5af78e'
    yellow:  '#f4f99d'
    blue:    '#caa9fa'
    magenta: '#ff92d0'
    cyan:    '#9aedfe'
    white:   '#e6e6e6'
```

#### Kitty
```conf
# ~/.config/kitty/kitty.conf
background #282a36
foreground #f8f8f2
color0  #000000
color1  #ff5555
color2  #50fa7b
color3  #f1fa8c
color4  #bd93f9
color5  #ff79c6
color6  #8be9fd
color7  #bfbfbf
color8  #4d4d4d
color9  #ff6e67
color10 #5af78e
color11 #f4f99d
color12 #caa9fa
color13 #ff92d0
color14 #9aedfe
color15 #e6e6e6
```

#### iTerm2
1. Download theme files from [iTerm2-Color-Schemes](https://github.com/mbadolato/iTerm2-Color-Schemes)
2. Import via Preferences → Profiles → Colors → Color Presets
3. Select your preferred theme

#### WezTerm
```toml
# ~/.config/wezterm/wezterm.lua
local config = {}

config.color_scheme = "Dracula"

return config
```

## Application Configuration

### Theme Settings

The application supports theme configuration through the config file:

```yaml
# ~/.config/proxmox-tui/config.yml
theme:
  # Enable terminal emulator color scheme adaptation (recommended)
  use_terminal_colors: true

  # Color scheme: "auto" (default), "light", "dark"
  color_scheme: auto
```

### Configuration Options

#### `use_terminal_colors`
- **`true`** (default): Use semantic colors that adapt to terminal theme
- **`false`**: Use fixed ANSI colors regardless of terminal theme

#### `color_scheme`
- **`auto`** (default): Automatically detect light/dark theme
- **`light`**: Force light theme colors
- **`dark`**: Force dark theme colors

## Color Semantics

The application uses semantic color constants that maintain consistent meaning across themes:

### Status Colors
- **Green**: Running VMs, online nodes, successful operations
- **Red**: Stopped VMs, offline nodes, errors
- **Yellow**: Pending operations, warnings, partial failures
- **Blue**: Informational elements, descriptions

### Resource Usage Colors
- **Green**: Low usage (< 50%)
- **Yellow**: Medium usage (50-75%)
- **Red**: High usage (75-90%)
- **Red (bright)**: Critical usage (> 90%)

### UI Element Colors
- **Primary**: Main text and important information
- **Secondary**: Supporting text and labels
- **Border**: Separators and borders
- **Selection**: Selected items and focus indicators

## Troubleshooting

### Colors Not Adapting to Terminal Theme

1. **Check terminal emulator settings**: Ensure your terminal is using the correct color scheme
2. **Verify configuration**: Check that `use_terminal_colors: true` in your config
3. **Restart application**: Some changes require a full restart
4. **Check TERM variable**: Ensure `TERM` is set correctly (e.g., `xterm-256color`)

### Poor Contrast or Readability

1. **Try different themes**: Some themes work better than others
2. **Adjust terminal colors**: Modify your terminal's color scheme
3. **Use high contrast themes**: Themes designed for accessibility often work well
4. **Check color scheme setting**: Try forcing `light` or `dark` in config

### Specific Color Issues

If certain colors appear wrong or are hard to read:

1. **Check terminal color support**: Ensure your terminal supports 256 colors
2. **Verify theme compatibility**: Some themes may not work well with TUI applications
3. **Report issues**: Open an issue with your terminal emulator and theme details

## Advanced Configuration

### Custom Color Schemes

For advanced users, you can create custom color schemes by modifying your terminal emulator's color palette. The application will automatically adapt to your custom colors.

### Environment Variables

The application respects standard terminal environment variables:
- `TERM`: Terminal type (should be `xterm-256color` or similar)
- `COLORTERM`: Terminal color support (e.g., `truecolor`)

## Best Practices

1. **Use established themes**: Popular themes are well-tested and work reliably
2. **Test with your workflow**: Ensure colors work well with your typical usage patterns
3. **Consider accessibility**: High contrast themes improve readability for many users
4. **Keep it simple**: Avoid overly complex color schemes that may cause issues

## Contributing

If you find a theme that works particularly well or have suggestions for improving the theming system, please:

1. Test with multiple terminal emulators
2. Document your findings
3. Submit a pull request with improvements
4. Include screenshots if possible

## References

- [Terminal Color Schemes](https://github.com/mbadolato/iTerm2-Color-Schemes)
- [Base16 Theme Standard](https://github.com/chriskempson/base16)
- [tcell Color Documentation](https://pkg.go.dev/github.com/gdamore/tcell/v2#Color)
- [tview Theme Documentation](https://pkg.go.dev/github.com/rivo/tview#Theme)
