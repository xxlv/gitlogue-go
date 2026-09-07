# gitlogue TUI demo.
# __COMMIT__ is filled by scripts/record.sh (default: HEAD).
# Usage: ./scripts/record.sh [revision]

Output docs/demo.gif
Output docs/demo.mp4

Require gitlogue

Set Shell "bash"
Set FontSize 16
Set Width 1400
Set Height 800
Set Theme "Dracula"
Set TypingSpeed 40ms
Set Framerate 30
Set Padding 16
Set WindowBar Colorful

Type "gitlogue --speed 2.0 __COMMIT__"
Sleep 400ms
Enter

Wait+Screen@30s /gitlogue-go/
Sleep 10s

# Faster, then browse the file tree and replay a file
Type "k"
Sleep 2s
Tab
Sleep 1s
Tab
Sleep 1s
Space
Sleep 5s

Type "q"
Sleep 400ms
