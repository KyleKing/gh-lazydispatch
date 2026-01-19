# VHS demo for workflow chains feature
#
# Run with:
#  vhs < .github/assets/chains-demo.tape
#
# This demo shows:
# - Viewing configured chains in the Chains tab
# - Selecting and executing a chain
# - Chain step details

Output .github/assets/chains-demo.gif
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
Type `clear`  Enter
Show

# Start the TUI
Type `../gh-lazydispatch`  Sleep 250ms  Enter
Sleep 1.5s

# Status bar shows Chains(3) badge
Sleep 500ms

# Focus right panel
Tab
Sleep 500ms

# Switch to Chains tab
Type `l`
Sleep 1s

# Navigate through chains
Down 1
Sleep 500ms
Down 1
Sleep 500ms
Up 1
Sleep 500ms
Up 1
Sleep 500ms

# Select first chain (deploy-all) to show details
Sleep 1s

# Show chain description and step count
Sleep 1s

# Return to History tab
Type `h`
Sleep 500ms

# Return focus to workflows
Tab
Tab
Sleep 500ms

# Quit
Type `q`
Sleep 250ms
