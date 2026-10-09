#!/bin/bash
set -euo pipefail

# This installs/removes Homebrew packages. Run only on a disposable CI runner.
if [[ "${GITHUB_ACTIONS:-}" != true || "$(uname -s)" != Darwin ]]; then
  echo "This smoke test requires a disposable macOS GitHub Actions runner." >&2
  exit 1
fi

mak_smoke_root=$(mktemp -d "${RUNNER_TEMP}/mak-dev-smoke.XXXXXX")
mak_smoke_binary="$(pwd)/bin/mak"
mak_smoke_home="$mak_smoke_root/home"
mkdir -p "$mak_smoke_home"
mak_smoke_cli=(env HOME="$mak_smoke_home" SHELL=/bin/zsh
  XDG_CONFIG_HOME="$mak_smoke_root/config" XDG_DATA_HOME="$mak_smoke_root/share"
  XDG_STATE_HOME="$mak_smoke_root/state" XDG_CACHE_HOME="$mak_smoke_root/cache"
  ZDOTDIR="$mak_smoke_home" NVIM_APPNAME=nvim "$mak_smoke_binary")

brew list --formula -1 | sort > "$mak_smoke_root/formulae-before"
brew list --cask -1 | sort > "$mak_smoke_root/casks-before"
for mak_smoke_kind in config share state cache; do
  mkdir -p "$mak_smoke_root/$mak_smoke_kind/nvim"
  printf 'Original %s\n' "$mak_smoke_kind" > "$mak_smoke_root/$mak_smoke_kind/nvim/original"
done
mak_smoke_ghostty_macos="$mak_smoke_home/Library/Application Support/com.mitchellh.ghostty"
for mak_smoke_ghostty_dir in "$mak_smoke_root/config/ghostty" "$mak_smoke_ghostty_macos"; do
  mkdir -p "$mak_smoke_ghostty_dir"
  printf 'theme = original\n' > "$mak_smoke_ghostty_dir/config.ghostty"
done

"${mak_smoke_cli[@]}" setup dev --yes
eval "$("${mak_smoke_cli[@]}" shellenv)"
cmp internal/devenv/assets/ghostty/config "$mak_smoke_ghostty_macos/config"
/Applications/Ghostty.app/Contents/MacOS/ghostty +validate-config --config-file="$mak_smoke_ghostty_macos/config"
"${mak_smoke_cli[@]}" doctor
"${mak_smoke_cli[@]}" setup dev --uninstall

for mak_smoke_kind in config share state cache; do
  test "$(cat "$mak_smoke_root/$mak_smoke_kind/nvim/original")" = "Original $mak_smoke_kind"
  test "$(ls -A "$mak_smoke_root/$mak_smoke_kind/nvim" | wc -l | tr -d ' ')" = 1
done
for mak_smoke_ghostty_dir in "$mak_smoke_root/config/ghostty" "$mak_smoke_ghostty_macos"; do
  test "$(cat "$mak_smoke_ghostty_dir/config.ghostty")" = 'theme = original'
  test "$(ls -A "$mak_smoke_ghostty_dir" | wc -l | tr -d ' ')" = 1
done
test ! -e "$mak_smoke_root/state/mak/dev-environment.json"
test ! -e "$mak_smoke_root/state/mak/dev-setup-journal.json"
brew list --formula -1 | sort > "$mak_smoke_root/formulae-after"
brew list --cask -1 | sort > "$mak_smoke_root/casks-after"
diff -u "$mak_smoke_root/formulae-before" "$mak_smoke_root/formulae-after"
diff -u "$mak_smoke_root/casks-before" "$mak_smoke_root/casks-after"
echo "Install, doctor, uninstall, original-file restoration, and package ownership passed."
