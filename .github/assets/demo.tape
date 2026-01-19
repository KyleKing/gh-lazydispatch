# VHS documentation
#
# Run with:
#  vhs < .github/assets/demo.tape
#
# Output:
#   Output <path>.gif               Create a GIF output at the given <path>
#   Output <path>.mp4               Create an MP4 output at the given <path>
#   Output <path>.webm              Create a WebM output at the given <path>
#
# Settings:
#   Set FontSize <number>           Set the font size of the terminal
#   Set FontFamily <string>         Set the font family of the terminal
#   Set Height <number>             Set the height of the terminal
#   Set Width <number>              Set the width of the terminal
#   Set LetterSpacing <float>       Set the font letter spacing (tracking)
#   Set LineHeight <float>          Set the font line height
#   Set Theme <string>              Set the theme of the terminal (JSON)
#   Set Padding <number>            Set the padding of the terminal
#   Set Framerate <number>          Set the framerate of the recording
#   Set PlaybackSpeed <float>       Set the playback speed of the recording
#
# Sleep:
#   Sleep <time>                    Sleep for a set amount of <time> in seconds
#
# Type:
#   Type[@<time>] "<characters>"    Type <characters> into the terminal with a
#                                   <time> delay between each character
#
# Keys:
#   Backspace[@<time>] [number]     Press the Backspace key
#   Down[@<time>] [number]          Press the Down key
#   Enter[@<time>] [number]         Press the Enter key
#   Space[@<time>] [number]         Press the Space key
#   Tab[@<time>] [number]           Press the Tab key
#   Left[@<time>] [number]          Press the Left Arrow key
#   Right[@<time>] [number]         Press the Right Arrow key
#   Up[@<time>] [number]            Press the Up Arrow key
#   Down[@<time>] [number]          Press the Down Arrow key
#   Ctrl+<key>                      Press the Control key + <key> (e.g. Ctrl+C)
#
# Display:
#   Hide                            Hide the subsequent commands from the output
#   Show                            Show the subsequent commands in the output

Output .github/assets/demo.gif
Set Shell bash
Set PlaybackSpeed 0.5
Set TypingSpeed 10ms

Set Theme "Catppuccin Macchiato"
Set FontFamily "Hack Nerd Font Mono"

Set FontSize 14
Set Width 1400
Set Height 900

Hide
Type `cd ~/Developer/kyleking/gh-lazydispatch/testdata`  Enter  Sleep 250ms
Type '../gh-lazydispatch 2>/dev/null &'  Enter  Sleep 500ms
Type 'fg'  Enter  Sleep 250ms
Type `clear`  Enter
Show

# Start the TUI (shows 'all' by default)
Type `../gh-lazydispatch`  Sleep 250ms  Enter
Sleep 1.5s

# Show default 'all' selection with tabbed right panel
Sleep 500ms

# Focus right panel and switch tabs to show Chains
Tab
Sleep 500ms
Type `l`
Sleep 750ms

# Show Live tab
Type `l`
Sleep 750ms

# Back to History tab
Type `h`
Sleep 500ms
Type `h`
Sleep 500ms

# Return focus to workflows pane
Tab
Tab
Sleep 500ms

# Navigate down to first workflow
Down 1
Sleep 500ms

# Use Space to select workflow and jump to config
Space
Sleep 750ms

# Edit version input using number shortcut
Type `1`
Sleep 500ms
Ctrl+u
Type `v1.2.3`
Sleep 250ms
Enter
Sleep 750ms

# Edit environment choice using number shortcut
Type `2`
Sleep 500ms
Down 1
Sleep 250ms
Enter
Sleep 750ms

# Toggle dry-run boolean using number shortcut
Type `3`
Sleep 500ms
Type `y`
Sleep 250ms
Enter
Sleep 750ms

# Show the command preview
Sleep 1s

# Navigate back to workflow pane to show navigation
Escape
Sleep 500ms
Down 1
Sleep 500ms

# Use Space again to select second workflow
Space
Sleep 500ms

# Show test workflow inputs briefly
Sleep 750ms

# Quit without executing
Type `q`
Sleep 250ms
