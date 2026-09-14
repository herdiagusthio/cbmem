# cbmem Installation

**Type:** How-to — get cbmem working end-to-end.

## Option 1: Binary (fast)

```bash
# Download latest
curl -L https://github.com/herdiagusthio/cbmem/releases/download/v0.1.0/cbmem-linux-amd64 -o ~/.local/bin/cbmem
chmod +x ~/.local/bin/cbmem

# Verify
cbmem version
# -> cbmem v0.1.0-dev
```

## Option 2: Build from source

```bash
git clone https://github.com/herdiagusthio/cbmem.git
cd cbmem
go build -o cbmem .
sudo mv cbmem ~/.local/bin/
cbmem version
```

Requires: Go 1.22+, GCC (for CGO mattn/go-sqlite3).

## Install Hermes Skill

```bash
# Copy skill to Hermes
cp -r ~/.hermes/skills/software-development/codebase-onboard ~/.hermes/skills/software-development/

# Or from repo:
bash scripts/install-git-hooks.sh  # optional: installs pre-commit/post-merge

# Verify:
ls ~/.hermes/skills/software-development/codebase-onboard/{scripts,templates,testdata,SKILL.md}
```

## Initialize First Repo

```bash
# Create output directory
mkdir -p ~/code-storage/second-brain/codebase/flight-search-system

# Index
cbmem index /path/to/repo --json
# -> {"repo":"flight-search-system","indexed_symbols":655,"indexed_edges":124,"indexed_files":41}

# Build links
cbmem links /path/to/repo
# -> 3 code + 0 adr + 3 notes -> links.jsonl

# Check staleness
cbmem stale /path/to/repo
# -> no stale links
```

## CI/CD Integration

Add `.github/workflows/cbmem-index.yml` (fork template). On push to main, it re-indexes self and commits `codebase/<repo>/` artifacts.

## Uninstall

```bash
rm -rf ~/.hermes/skills/software-development/codebase-onboard
rm -f ~/.local/bin/cbmem
```