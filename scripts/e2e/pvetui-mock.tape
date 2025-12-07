Set Shell "bash"
Set Width 120
Set Height 120
Set TypingSpeed 80ms
Output "scripts/e2e/output/pvetui-mock.txt"

# Launch pvetui against the mock profile provided by the runner.
Type "PVETUI_CONFIG=${PVETUI_CONFIG} ./bin/pvetui --profile mock\n"
Sleep 2000ms

# Quit via global quit + confirm
Type "q"
Sleep 200ms
Type "\n"
Sleep 500ms
