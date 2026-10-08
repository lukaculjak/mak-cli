# mak

`mak` is a personal developer CLI for installing a coding environment, generating validation composables, opening
recurring meetings, and managing encrypted browser-login prefills.
It is opinionated around Luka's own setup; anyone is welcome to use that same
environment. It is not intended as a configurable environment manager.

## Install

macOS and Linux releases are available for AMD64 and ARM64:

```sh
curl -fsSL https://raw.githubusercontent.com/lukaculjak/mak-cli/main/scripts/install.sh | bash
```

Run `mak --help` to see all commands.

Interactive terminal output uses orange help headings, command names, flags,
prompts, and setup steps, with labeled success, warning, and error messages.
Command separators make each run easier to distinguish from your shell prompt.
True-color terminals use orange `#ff9500`; 256-color terminals use orange 208,
with a yellow fallback for basic ANSI terminals. Ghostty settings are unchanged.

Colors and terminal separators are disabled when output is redirected, when
`TERM=dumb`, or when `NO_COLOR` is nonempty. To disable colors explicitly:

```sh
NO_COLOR=1 mak setup --help
```

Shell-completion scripts, `mak shellenv`, and browser native-messaging responses stay free of
styling and command separators. The shell still controls the prompt and the
commands you type; mak styles its own output.

## Commands

### Coding environment (macOS)

Close Neovim, then run as your normal user:

```sh
mak setup dev
```

The command asks `Install the development environment? [Y/n]:`. Press Enter
for yes or enter `no` to cancel before anything is installed or replaced. A hint
above the prompt points to the package list. To explicitly skip this confirmation
(for example, in a script), use `mak setup dev --yes` or `-y`. Missing stdin is
an error rather than an automatic yes. Homebrew/sudo may still request input.

`mak setup codeenv` and `mak setup codingenv` are aliases. This installs Homebrew
if needed, Neovim, Git, Node/npm, Go, Python, Ruby, Elixir/Erlang, GHC/Cabal/Haskell
Language Server, ripgrep, fd, fzf, lazygit, Tree-sitter, unzip, and JetBrains Mono
Nerd Font. Homebrew's installer may ask for your administrator password and
install Apple's developer tools. If the C compiler is still missing, mak opens
Apple's installation dialog and waits up to 20 minutes for you to finish it.
Use a supported macOS/Homebrew combination and an internet connection.

The command installs Luka's bundled LazyVim configuration at `~/.config/nvim`
(or `$XDG_CONFIG_HOME/nvim`). It includes Gruvbox with hard contrast, Emmet,
CSS/SCSS, HTML, JSON, JavaScript/TypeScript/TSX, Vue, Go, Python, Ruby, Haskell, and
Elixir support, the existing navigation and explorer customizations, and disabled
automatic formatting. Ruby LSP uses Mason instead of a laptop-specific rbenv
path; HLS uses the installed toolchain instead of a hardcoded GHCup path.
Go uses Mason's `gopls` instead of requiring `~/go/bin/gopls`, with syntax parsers
for Go, `go.mod`, `go.work`, and `go.sum`.

Setup waits for plugins, language servers, and syntax parsers to finish, checks
every enabled LSP can initialize, and exercises Go and TypeScript completion and
go-to-definition. Existing Neovim configuration, data, state, and cache are
replaced automatically after an informational message; the previous directories
are retained as adjacent `nvim.mak-backup-*` backups. Existing dotfiles symlinks
are backed up as symlinks; their targets are left intact. XDG directory overrides
are respected. Run no other Homebrew installations concurrently with setup.

On failure or Ctrl-C, mak removes the incomplete Neovim installation, restores
the old directories, and removes Homebrew formulae/casks introduced by the run,
including their newly installed formula dependencies. Existing packages are not
upgraded or uninstalled. Homebrew itself, Apple's developer tools, and package
download caches remain shared system prerequisites. If rollback fails, the error
identifies what still needs attention and the backup locations. A durable journal
at `$XDG_STATE_HOME/mak/dev-setup-journal.json` (default `~/.local/state/mak`)
records package inventories and planned backups before changes. After power loss
or SIGKILL, close Neovim and rerun the setup/install/remove command with the same
XDG settings. mak automatically restores an incomplete installation, or finishes
saving the ownership record if setup already passed verification. Interrupted
cleanup retains its checkpoints so the next invocation can continue.

The setup lock is a persistent file held by the process and inherited by its
installation subprocesses. It releases automatically when those processes exit;
do not delete it. Legacy lock **directories** from older mak versions still need
inspection before manual removal. Avoid other Homebrew installs until recovery
completes, since introduced packages are identified against the saved inventory.
Keep original backups and the recovery journal until recovery completes.

Open a new terminal after setup. For zsh/bash, mak adds Homebrew's `shellenv`
line to `.zprofile`/`.bash_profile` without replacing your shell settings (zsh's
`ZDOTDIR` is respected). mak refreshes PATH for its own installation processes;
it cannot change the parent shell's environment. To refresh the current zsh/bash
session immediately, run:

```sh
eval "$(mak shellenv)"
```

`mak shellenv` prints shell code; `eval` applies it in the shell you are already
using. It loads Homebrew's environment, adds installed Ruby/Python tool paths and
Homebrew's executable directories before system paths, and clears cached command
locations. Existing user/project paths stay first, preserving activated virtual
environments and runtimes selected through nvm/rbenv.
When run directly in a terminal, it prints an orange reminder on stderr explaining
how to apply the settings (including `eval "$(go run . shellenv)"` from source).
The reminder is omitted when stdout is captured or redirected, keeping `eval` quiet
and shell-code output clean. Normal `NO_COLOR` and terminal color rules apply.
Existing PATH entries are preserved; repeating it does not duplicate the managed
directories. It does not install software, edit profiles, or reload unrelated
shell settings. Other shells need their own Homebrew PATH setup. You can still
reload your full profile with `source "${ZDOTDIR:-$HOME}/.zprofile"` (zsh) or
`source "$HOME/.bash_profile"` (bash).
Select **JetBrainsMono Nerd Font** in your terminal's font
settings, then run `nvim`. Project dependencies, Python virtual environments,
Ruby bundles, and non-baseline GHC/HLS versions remain project-specific.

The configuration and `lazy-lock.json` live under `internal/devenv/assets/nvim`
and are embedded in the mak executable, so a separate dotfiles checkout is not
required on the new laptop. Setup installs the plugin commits in that lockfile;
it does not run `Lazy sync` or select the newest LazyVim automatically. See
[lazy.nvim's lockfile documentation](https://lazy.folke.io/usage/lockfile).
Homebrew runtimes, Mason tools, and their first-run dependencies follow their
package registries and are not version-locked by `lazy-lock.json`.

To change the environment shipped with mak, edit the bundled files, deliberately
update and test the plugin lockfile in an isolated Neovim environment, then build
or release mak again. `mak update` updates the executable; rerun `mak setup dev`
to apply its bundled environment. Local Neovim edits are replaced on that rerun,
with another backup retained. Running `:Lazy update` yourself changes your local
plugin versions; the next mak setup restores the bundled versions.

Preview all offered software without installing anything:

```sh
mak setup dev --list  # or -l
```

The checklist uses a green `[x]` for installed Homebrew packages and a red `[ ]`
for missing packages, with orange headings. It shows both preexisting software
and software installed by mak; `(tracked by mak)` indicates removable ownership.
Colors follow the normal `NO_COLOR`, terminal, and redirected-output rules.
Neovim's checkbox indicates whether the application is installed, not whether
the LazyVim environment has been fully configured. Software installed outside
Homebrew is not detected. Homebrew aliases such as Python's versioned formula
name are resolved and recorded so the correct package is listed and removed.

Manage one offered package by the name printed in the list:

```sh
mak setup dev --install node  # or -i node
mak setup dev --remove node   # or -r node
mak setup dev -i nvim         # full LazyVim environment, with confirmation
mak setup dev -r nvim         # Neovim only, restoring its original files
mak setup dev -i font         # JetBrains Mono Nerd Font
```

`nvim` is an alias for `neovim`, and `font` is an alias for
`font-jetbrains-mono-nerd-font`. Installing Neovim runs the complete environment
setup, including the bundled configuration, language runtimes, plugins, LSPs,
and parsers. Other individual installs add only the selected Homebrew package
and its dependencies, retaining ownership for later cleanup. Already installed
packages are left as is and are not newly claimed by mak.

Single-package removal refuses untracked packages and packages required by other
installed Homebrew software. It keeps the selected package's dependencies and
all other coding tools; `--uninstall` can clean up the remaining tracked packages.
Removing a tracked Neovim also removes its managed files, including local edits,
and restores the original backups. Removing other packages leaves Neovim files
alone; removing a runtime can disable language servers that need it. Failed
removals retain progress so the same `--remove PACKAGE` command can resume.
The list, install, remove, and full-uninstall flags are mutually exclusive.

Check the coding environment without installing, updating, or repairing it:

```sh
mak doctor          # alias: mak healthcheck
```

Doctor reports tools available on PATH, Apple developer tools, Homebrew packages,
installation tracking and recovery files, bundled configuration changes, locked
plugin revisions, syntax parsers, completion capabilities, and language-server
startup. Green success messages, orange warnings, and red errors include suggested
repair commands where needed. Repairs are never run automatically. The final line
prints the current mak version. Failed checks exit with status 1; warnings alone
exit with status 0.

Neovim checks run against the installed plugins and tools with temporary copies of
configuration and ElixirLS caches. The macOS sandbox blocks downloads and writes
outside the temporary directory; scratch files are removed afterward. A running
setup/removal skips runtime checks with a warning. Diagnostics check a temporary
sample project, so project-specific dependencies or GHC/HLS compatibility may
still need attention in your actual project. Font installation is checked;
selecting the font in your terminal remains manual.

To remove a tracked coding environment, close Neovim and run:

```sh
mak setup dev --uninstall
```

This removes the managed Neovim configuration, plugins, language servers, state,
and cache, including local edits, then restores the files or symlinks from before
the first tracked setup. With no previous Neovim installation, those directories
are removed. It removes only Homebrew formulae, dependencies, and casks introduced
by mak, keeping preexisting packages and packages now required by other installed
tools. Homebrew, Apple's developer tools, package download caches, and intermediate
backups from repeated setups remain. Only shell `shellenv` blocks added by tracked
setups are removed; other shell settings and later edits are preserved.

Setup saves its recovery record at `~/.local/state/mak/dev-environment.json`
(or `$XDG_STATE_HOME/mak/dev-environment.json`). Keep this file, the adjacent
Neovim backups, and the same XDG settings until uninstall completes. A failed
uninstall retains its progress so the same command can resume without deleting
already restored files. After power loss or SIGKILL, rerun the same command;
the process lock releases automatically and removal resumes from its checkpoints.

Installations made before recovery tracking was added cannot be automatically
uninstalled: mak reports the missing record and leaves Neovim files and packages
alone. Rerunning setup starts tracking from the current environment; it cannot
recover ownership of packages introduced by an older, untracked setup.

### Validation composables

From the root of a Quasar or Nuxt 4 project:

```sh
mak setup validation
```

The command creates `useForm.ts` and `useValidationRules.ts` under
`src/composables` for Quasar or `app/composables` for Nuxt 4. Existing files
are never overwritten.

### Recurring meetings

```sh
mak meet add
mak meet list
mak meet open <alias>
mak meet edit <alias>
mak meet delete <alias>
```

Meetings are stored in `~/.config/mak/meetings.json`. Recurring schedules are
installed in the current user's crontab and open links with the system browser.
Add/edit/delete only report success after both the meeting file and schedules are
saved. A failed cron update leaves the meeting file unchanged; a failed file save
restores the previous cron. Each successful change reconciles all managed meeting
schedules while preserving unrelated cron entries. If restoring cron fails, mak
returns an error with recovery instructions.

### Credential prefills

```sh
mak prefill add
mak prefill list
mak prefill edit <project-name>
mak prefill delete <project-name>
mak prefill install
```

After `mak prefill install`, load the generated directory as an unpacked
Chrome, Brave, or Edge extension. Copy its extension ID and authorize it:

```sh
mak prefill install --extension-id <extension-id>
```

Credentials are encrypted at rest with a key derived from the master password
using scrypt and AES-256-GCM. The decrypted credentials are held in the
browser's extension session storage after unlocking and are cleared when the
extension is locked or the browser session ends. `mak` has no password-recovery
mechanism, so back up `~/.config/mak/prefills.enc` and retain the master password.

## Updating and uninstalling

```sh
mak update
mak uninstall
mak uninstall --purge
```

Updates are installed only when the published semantic version is newer and
the archive matches the release checksum. A normal uninstall preserves user
data. `--purge` additionally removes mak configuration, credentials, native
messaging manifests, and managed meeting cron jobs.

Automatic update notifications cache release checks for 24 hours, including
failed checks, and allow at most 250 ms for a network request. The explicit
`mak update` command always checks for the latest release independently.

## Development

The project requires the Go version declared in `go.mod`.

```sh
go test ./...
go vet ./...
go build ./...
```

Tags matching `v*` trigger the GitHub Actions release workflow and GoReleaser.
CI runs vet, race tests, and builds on Linux and macOS. Releases also require a
disposable macOS runner to install the bundled environment in an isolated home,
run doctor, uninstall it, and verify original files and preexisting packages.
The same smoke test can be launched through the Development environment smoke
test workflow. `scripts/test-dev-environment.sh` is for disposable macOS GitHub
Actions runners only; it installs and removes Homebrew packages.

## License

This project is available under the [MIT License](LICENSE).
