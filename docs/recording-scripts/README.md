# Video Recording Scripts

Scripts for recording Relicta documentation videos using asciinema.

## Setup

```bash
# Install asciinema and agg (for GIF conversion)
brew install asciinema
cargo install --git https://github.com/asciinema/agg

# Ensure relicta is in PATH
which relicta

# Create a demo repository
./setup-demo-repo.sh
```

## Recording

**Important**: Recording must be done in an interactive terminal (not headless).

### Method 1: Interactive Recording (Recommended)

Open a real terminal and run:

```bash
# Set up demo repo first
cd /path/to/relicta/docs/recording-scripts
./setup-demo-repo.sh

# Start recording interactively
asciinema rec /tmp/output.cast --cols 80 --rows 24 --idle-time-limit 2

# Now manually run the script commands or source the script
source ./cmd-plan.sh

# Press Ctrl+D when done
```

### Method 2: Script Recording

```bash
# Record a script (requires interactive terminal)
asciinema rec output.cast --command "bash script.sh" --cols 80 --rows 24 --idle-time-limit 2
```

### Converting Recordings

```bash
# Convert to GIF
agg output.cast output.gif --font-size 14 --theme monokai

# Convert to MP4 (requires ffmpeg)
ffmpeg -i output.gif -movflags faststart -pix_fmt yuv420p -vf "scale=trunc(iw/2)*2:trunc(ih/2)*2" output.mp4

# Copy to website
cp output.gif output.mp4 /path/to/relicta-site/docs/public/videos/
```

## Scripts

### MCP Integration
- `mcp-demo.sh` - AI-managed releases with Claude

### CLI Commands (Success + Error)
- `cmd-release.sh` - One-command release workflow
- `cmd-plan.sh` - Analyze commits and plan release
- `cmd-bump.sh` - Version bump
- `cmd-notes.sh` - Generate release notes
- `cmd-approve.sh` - Approve release
- `cmd-publish.sh` - Publish release
- `cmd-status.sh` - View release status
- `cmd-cancel.sh` - Cancel active release
- `cmd-clean.sh` - Clean old releases
- `cmd-blast.sh` - Blast radius analysis
- `cmd-policy.sh` - Policy validation
- `cmd-serve.sh` - Dashboard server

### Quickstart
- `quickstart.sh` - Install to first release

### Feature Demos
- `demo-ai-vs-template-notes.sh` - Compare AI-powered vs template release notes
- `demo-conventional-vs-messy-commits.sh` - Show Relicta working with messy commit history
