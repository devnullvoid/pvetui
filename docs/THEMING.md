# Theming Guide

Proxmox TUI supports flexible theming through both terminal emulator color schemes and application-level configuration.

## Overview

The application uses semantic color constants that automatically adapt to your terminal emulator's color scheme. This approach provides the best user experience by:

- **Leveraging existing terminal themes**: Use your favorite themes (Dracula, Nord, Solarized, etc.)
- **Maintaining semantic meaning**: Colors always represent the same concepts regardless of theme
- **Minimal configuration**: Works out of the box with most terminal emulators
- **Consistent experience**: Same theme across all terminal applications

## Application-Level Theming

You can override any semantic color in your config file. This allows for precise control over the UI appearance, regardless of your terminal's palette.

### How it works
- By default, semantic colors (Primary, Success, Warning, etc) use your terminal's ANSI palette.
- You can override any color by specifying a hex code (e.g. `#1e1e2e`), an ANSI color name (e.g. `green`, `yellow`), or a W3C color name (e.g. `mediumseagreen`).
- The special color name `default` uses your terminal's default color.
- Any omitted color key will use the built-in default for that semantic role.

### All Themeable Color Keys
- `primary`, `secondary`, `tertiary`, `success`, `warning`, `error`, `info`
- `background`, `border`, `selection`, `header`, `headertext`, `footer`, `footertext`
- `title`, `contrast`, `morecontrast`, `inverse`
- `statusrunning`, `statusstopped`, `statuspending`, `statuserror`
- `usagelow`, `usagemedium`, `usagehigh`, `usagecritical`

### Example config
```yaml
theme:
  colors:
    primary: "#e0e0e0"
    secondary: "gray"
    tertiary: "aqua"
    success: "green"
    warning: "yellow"
    error: "red"
    info: "blue"
    background: "default"
    border: "gray"
    selection: "blue"
    header: "navy"
    headertext: "yellow"
    footer: "black"
    footertext: "white"
    title: "white"
    contrast: "blue"
    morecontrast: "fuchsia"
    inverse: "black"
    statusrunning: "green"
    statusstopped: "maroon"
    statuspending: "yellow"
    statuserror: "red"
    usagelow: "green"
    usagemedium: "yellow"
    usagehigh: "red"
    usagecritical: "fuchsia"
```

- Any omitted color key will use the built-in default for that semantic role.
- You can use any valid tcell color name or hex code.

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
2. **Verify configuration**: Check your theme color overrides in the config
3. **Restart application**: Some changes require a full restart
4. **Check TERM variable**: Ensure `TERM` is set correctly (e.g., `xterm-256color`)

### Poor Contrast or Readability

1. **Try different themes**: Some themes work better than others
2. **Adjust terminal colors**: Modify your terminal's color scheme
3. **Use high contrast themes**: Themes designed for accessibility often work well

### Specific Color Issues

If certain colors appear wrong or are hard to read:

1. **Check terminal color support**: Ensure your terminal supports 256 colors or truecolor
2. **Verify theme compatibility**: Some themes may not work well with TUI applications
3. **Report issues**: Open an issue with your terminal emulator and theme details

## Advanced Configuration

### Custom Color Schemes

For advanced users, you can create custom color schemes by modifying your terminal emulator's color palette. The application will automatically adapt to your custom colors unless you override them in the config.

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
