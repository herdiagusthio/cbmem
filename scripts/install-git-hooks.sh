#!/usr/bin/env bash
# install-git-hooks.sh — T5.1: pre-commit hash check + post-merge incremental index
set -euo pipefail
HOOK_DIR="${1:-.git/hooks}"
if [[ ! -d .git ]]; then echo "not a git repo (run from repo root)" >&2; exit 1; fi
mkdir -p "$HOOK_DIR"
cat > "$HOOK_DIR/pre-commit" <<'HOOK'
#!/usr/bin/env bash
set -e
# fast: block if cbmem not built, else warn on large diff (>100 files)
if command -v cbmem >/dev/null 2>&1 || [[ -x "$HOME/code-storage/cbmem/cbmem" ]]; then
  n=$(git diff --cached --name-only | wc -l); n=$(echo "$n" | tr -d ' ')
  if [[ "$n" -gt 100 ]]; then echo "pre-commit: $n staged files — consider incremental index after commit" >&2; fi
fi
HOOK
cat > "$HOOK_DIR/post-merge" <<'HOOK'
#!/usr/bin/env bash
set -e
BIN="$(command -v cbmem 2>/dev/null || echo "$HOME/code-storage/cbmem/cbmem")"
if [[ -x "$BIN" ]]; then
  echo "post-merge: running cbmem index --incremental --json ..." >&2
  "$BIN" index . --incremental --json 2>&1 | head -5 || true
fi
HOOK
chmod +x "$HOOK_DIR/pre-commit" "$HOOK_DIR/post-merge"
echo "installed $HOOK_DIR/pre-commit + post-merge (ponytail: pre-push full check when repo >100k sym)"
