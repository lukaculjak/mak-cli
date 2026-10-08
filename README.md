# mak

`mak` is a personal developer CLI for installing a coding environment, generating validation composables, opening
recurring meetings, and managing encrypted browser-login prefills.

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

Shell-completion scripts and browser native-messaging responses stay free of
styling and command separators. The shell still controls the prompt and the
commands you type; mak styles its own output.

## Commands

### Coding environment (macOS)

Close Neovim, then run as your normal user:

```sh
mak setup dev
```

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
identifies what still needs attention and the backup locations. An interrupted
process that cannot run cleanup (for example, power loss or SIGKILL) may leave
`~/.config/mak/dev-setup.lock`; inspect the backups before removing that lock and
retrying. Keep the original backups to allow restoration with `--uninstall`.

Open a new terminal after setup. For zsh/bash, mak adds Homebrew's `shellenv`
line to `.zprofile`/`.bash_profile` without replacing your shell settings (zsh's
`ZDOTDIR` is respected). Select **JetBrainsMono Nerd Font** in your terminal's font
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
already restored files. After a power loss or SIGKILL, inspect the backups and
remove the stale `dev-setup.lock` before retrying.

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

## Development

The project requires the Go version declared in `go.mod`.

```sh
go test ./...
go vet ./...
go build ./...
```

Tags matching `v*` trigger the GitHub Actions release workflow and GoReleaser.

## License

This project is available under the [MIT License](LICENSE).
