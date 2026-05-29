Set Shell "bash"
Set Width 120
Set Height 120
Set TypingSpeed 80ms
Output "scripts/e2e/output/pvetui-mock.raw.txt"

# Launch pvetui against the mock profile provided by the runner.
Hide
Type "env -u NO_COLOR TERM=xterm-256color COLORTERM=truecolor ./bin/pvetui --config $PVETUI_MEDIA_CONFIG --profile $PVETUI_MEDIA_PROFILE --no-cache"
Enter
Show
Sleep 5000ms

# Quit via global quit + confirm
Type "q"
Sleep 200ms
Enter
Sleep 500ms
