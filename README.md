# mak

`mak` is a personal developer CLI for installing a coding environment, generating validation composables, opening
recurring meetings, syncing Neorg notes, running Pomodoro timers, and managing encrypted browser-login prefills.
It is opinionated around Luka's own setup; anyone is welcome to use that same
environment. It is not intended as a configurable environment manager.

## Install

macOS and Linux releases are available for AMD64 and ARM64:

```sh
curl -fsSL https://raw.githubusercontent.com/lukaculjak/mak-cli/main/scripts/install.sh | bash
```

Run `mak --help` to see all commands.

For zsh users, the installer also enables mak tab completion in
`${ZDOTDIR:-$HOME}/.zshrc`. Open a new terminal after installation. For an existing
installation, enable it with:

```sh
mak completion zsh --install
```

This appends one block while preserving your shell settings, and needs no
Homebrew directory access or sudo. `mak completion zsh` by itself only prints
the generated script. To enable completion immediately in the current zsh session:

```sh
autoload -Uz compinit; compinit
source <(mak completion zsh)
```

Other shells retain the generated-script instructions under
`mak completion SHELL --help`.

Interactive terminal output uses orange help headings, command names, flags,
prompts, and setup steps, with labeled success, warning, and error messages.
Command separators make each run easier to distinguish from your shell prompt.
True-color terminals use orange `#ff9500`; 256-color terminals use orange 208,
with a yellow fallback for basic ANSI terminals. These output styles do not change terminal settings.

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

During installation, mak shows seven numbered stages and an orange spinner below
the live installation logs. The spinner pauses for Homebrew's interactive installer.
Successful commands using the normal terminal footer finish with
`[ok] Completed in MM:SS`, measured from command startup and including time spent
at prompts. Minutes can exceed 59 for long installations. Shell-code output,
completion scripts, and commands with their own footer keep their existing output.
Redirected output, `TERM=dumb`, and `NO_COLOR` use plain stage lines without animation.

`mak setup codeenv` and `mak setup codingenv` are aliases. This installs Homebrew
if needed, Neovim, Git, Node/npm, Go, Python, Ruby, Elixir/Erlang, GHC/Cabal/Haskell
Language Server, ripgrep, fd, fzf, lazygit, Tree-sitter library/CLI, unzip, LuaRocks, terminal-notifier, caarlos0/tap/timer,
Maple Mono NF, JetBrains Mono Nerd Font, Ghostty, and Meslo LG Nerd Font. Homebrew's installer may ask for your administrator password and
install Apple's developer tools. If the C compiler is still missing, mak opens
Apple's installation dialog and waits up to 20 minutes for you to finish it.
Use a supported macOS/Homebrew combination and an internet connection.
Setup checks Homebrew's managed-directory permissions and existing Cellar package
directories before installing packages. Doctor uses the same read-only checks.
If an existing Homebrew installation belongs to another account or has lost write
access, setup stops before changing Neovim or Ghostty files. Run
`brew doctor check_access_directories check_exist_directories` as your normal user
and follow the reported directory repairs if this account should manage Homebrew.
Do not run mak or brew with sudo. Permission repair may need sudo for the specific
directories listed in the repair prompt; mak never takes ownership of `/usr/local` itself.
Before installing a package, mak checks Homebrew and the commands on your `PATH`.
Existing tools from nvm, rbenv, GHCup, or other installers are reused without
claiming ownership. Packages with multiple required commands, such as Node.js
and npm, are skipped only when all are available. Ghostty is also detected in
`/Applications` and `~/Applications`. Fonts and libraries are checked through
Homebrew because they have no executable. Homebrew may still install dependencies
required by a missing package. Existing Homebrew packages are skipped, including
versioned aliases such as Python.
Missing packages are installed one at a time, stopping at the first failed package.
Homebrew source builds can take hours, especially on Intel Macs without compatible
prebuilt packages. Package installation has no fixed mak timeout; Ctrl+C cancels
the installation and starts rollback. The subsequent Neovim configuration phase
has a separate 60-minute limit. Cancellation and timeout errors name their cause.
Versioned Homebrew Python installations are added as fallbacks when `python3`
is not linked into Homebrew's main bin directory. Setup requires Python 3.10 or
newer with `venv` and `ensurepip` for Mason's Python tools and checks this before
replacing Neovim or Ghostty files. Project and version-manager paths retain priority.
Homebrew's `tree-sitter` package provides the library; `tree-sitter-cli` provides
the executable required by LazyVim. Setup installs both and checks the CLI before
replacing Neovim files or starting plugin installation.

The command installs Luka's bundled LazyVim configuration at `~/.config/nvim`
(or `$XDG_CONFIG_HOME/nvim`). It includes Duskfox by default (plus Melange, Kanagawa and Gruvbox), Emmet,
CSS/SCSS, HTML, JSON, JavaScript/TypeScript/TSX, Vue, Go, Python, Ruby, Haskell, and
Elixir, C/C++ and Tailwind support, Neorg, CSS color previews, Git line blame,
DAP debugging, the existing navigation and explorer customizations, and disabled
automatic formatting. Ruby LSP uses Mason instead of a laptop-specific rbenv
path; HLS uses the installed toolchain instead of a hardcoded GHCup path.
Go uses Mason's `gopls` instead of requiring `~/go/bin/gopls`, with syntax parsers
for Go, `go.mod`, `go.work`, and `go.sum`.

Setup also bundles Luka's Ghostty settings: niji theme, 88% background opacity,
Display P3, native title bar, 160×62 window, 10-pixel padding, size-16 Maple Mono NF,
shell integration, split navigation, close-surface bindings, and Shift+Enter input.
`Maple Mono NF` is the primary font, with `MesloLGS Nerd Font` as a fallback. The settings are written to
`~/Library/Application Support/com.mitchellh.ghostty/config`. Both that directory
and `$XDG_CONFIG_HOME/ghostty` (default `~/.config/ghostty`) are backed up and
replaced; the XDG directory contains a comment pointing to the macOS config.
This avoids old settings overriding or duplicating the bundled configuration.
Ghostty backups are adjacent `*.mak-backup-*` directories, including any existing
`config.ghostty` files. Open a new Ghostty window after setup.

Setup waits for plugins, language servers, and syntax parsers to finish, checks
every enabled LSP can initialize, and exercises Go and TypeScript completion and
go-to-definition. Existing Neovim configuration, data, state, and cache are
replaced automatically after an informational message; the previous directories
are retained as adjacent `nvim.mak-backup-*` backups. Existing dotfiles symlinks
are backed up as symlinks; their targets are left intact. XDG directory overrides
are respected. Run no other Homebrew installations concurrently with setup.

If an existing Homebrew installation is not writable by your account, setup
offers a separate permission repair prompt that defaults to no. It lists the
affected directories before asking for an explicit yes, even with `--yes`.
After approval, mak uses `sudo` to change ownership of those directories and their
contents to the current account, grants write access, and rechecks permissions
before installing packages. This can affect other accounts sharing Homebrew;
permission repairs remain after a setup failure or uninstall. Run mak as your
normal user, without `sudo`; the repair itself may request an administrator password.
Unknown diagnostics, missing directories, or paths outside the supported Homebrew
prefix need manual repair using the suggestions from
`brew doctor check_access_directories check_exist_directories`.

If Homebrew's package lookup fails with an unknown or unsupported macOS version,
setup runs `brew update` once and retries the lookup before taking package
inventories or replacing configuration. This refreshes the shared Homebrew client
and package definitions; it does not run `brew upgrade`. If the lookup still fails,
mak stops with diagnostic commands to run. `mak doctor` checks this lookup too,
but doctor and `--list` never update Homebrew.

On failure or Ctrl-C, mak removes the incomplete Neovim and Ghostty configuration, restores
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
Ghostty uses the bundled Maple Mono NF font selection automatically. For other terminals,
select **JetBrainsMono Nerd Font** in your terminal's font
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
to apply its bundled environment. Local Neovim and Ghostty edits are replaced on that rerun,
with another backup retained. Running `:Lazy update` yourself changes your local
plugin versions; the next mak setup restores the bundled versions.

Preview all offered software without installing anything:

```sh
mak setup dev --list  # or -l
```

The checklist uses a green `[x]` for available software and a red `[ ]`
for missing packages, with orange headings. It shows both preexisting software
and software installed by mak; `(tracked by mak)` indicates removable ownership.
Colors follow the normal `NO_COLOR`, terminal, and redirected-output rules.
Neovim's checkbox indicates whether the application is installed, not whether
the LazyVim environment has been fully configured. External executables show
their detected paths and are not tracked by mak. Homebrew aliases such as Python's versioned formula
name are resolved and recorded so the correct package is listed and removed.

Manage one offered package by the name printed in the list:

```sh
mak setup dev --install node  # or -i node
mak setup dev --remove node   # or -r node
mak setup dev -i nvim         # full LazyVim environment, with confirmation
mak setup dev -r nvim         # Neovim only, restoring its original files
mak setup dev -i font         # JetBrains Mono Nerd Font
mak setup dev -i ghostty      # Ghostty + bundled settings + Maple/Meslo fonts (with confirmation)
mak setup dev -r ghostty      # Ghostty only, restoring its original settings
mak setup dev -i tree-sitter-cli # Tree-sitter executable for syntax parsers
```

`nvim` is an alias for `neovim`, and `font` is an alias for
`font-jetbrains-mono-nerd-font`. Installing Neovim runs the complete environment
setup, including the bundled configuration, language runtimes, plugins, LSPs,
and parsers. Installing Ghostty adds the application, Maple Mono NF, Meslo LG Nerd Font, and
the bundled settings, with the same confirmation and backup handling as full
setup. Other individual installs add only the selected Homebrew package and its
dependencies, retaining ownership for later cleanup. Already installed packages
are not newly claimed by mak; Ghostty's settings are still applied and backed up.

Single-package removal refuses untracked packages and packages required by other
installed Homebrew software. It keeps the selected package's dependencies and
all other coding tools; `--uninstall` can clean up the remaining tracked packages.
Removing a tracked Neovim also removes its managed files, including local edits,
and restores the original backups. Removing Ghostty restores its original
configuration while retaining Neovim files. Removing other packages leaves
Neovim and Ghostty files alone; removing a runtime can disable language servers that need it. Failed
removals retain progress so the same `--remove PACKAGE` command can resume.
The list, install, remove, and full-uninstall flags are mutually exclusive.

Check the coding environment without installing, updating, or repairing it:

```sh
mak doctor          # alias: mak healthcheck
```

Doctor reports tools available on PATH, Apple developer tools, Homebrew packages,
write permissions for Homebrew's managed directories,
installation tracking and recovery files, bundled configuration changes, locked
plugin revisions, syntax parsers, completion capabilities, and language-server
startup. Green success messages, orange warnings, and red errors include suggested
repair commands where needed. Doctor never performs repairs. If Homebrew permissions
fail, resolve them first and rerun `mak setup dev`; missing tools and configuration
may be consequences of setup stopping before installation. The final line
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

This removes the managed Ghostty configuration and Neovim configuration,
plugins, language servers, state, and cache, including local edits, then restores
the files or symlinks from before the first tracked setup. With no previous
installation, managed directories are removed. It removes only Homebrew formulae, dependencies, and casks introduced
by mak, keeping preexisting packages and packages now required by other installed
tools. Homebrew, Apple's developer tools, package download caches, and intermediate
backups from repeated setups remain. Only shell `shellenv` blocks added by tracked
setups are removed; other shell settings and later edits are preserved.

Setup saves its recovery record at `~/.local/state/mak/dev-environment.json`
(or `$XDG_STATE_HOME/mak/dev-environment.json`). Keep this file, the adjacent
Neovim and Ghostty backups, and the same HOME/XDG settings until uninstall completes. A failed
uninstall retains its progress so the same command can resume without deleting
already restored files. After power loss or SIGKILL, rerun the same command;
the process lock releases automatically and removal resumes from its checkpoints.

Installations made before recovery tracking was added cannot be automatically
uninstalled: mak reports the missing record and leaves Neovim files and packages
alone. Rerunning setup starts tracking from the current environment; it cannot
recover ownership of packages introduced by an older, untracked setup.

### Neorg notes, iCloud Drive, and Pomodoro (macOS)

The full `mak setup dev` includes Neorg, its Lua dependencies, the Pomodoro tools,
and a journal reminder. To add a feature to an existing LazyVim installation:

```sh
mak setup neorg                # also: mak setup dev --neorg
mak setup pomodoro             # also: mak setup dev --pomodoro
mak work                       # 50 minutes, then a desktop notification
mak rest                       # 10 minutes, then a desktop notification
```

Neorg stores plain `.norg` files in `~/neorg`; its dotfile configuration is
`~/.config/nvim/lua/plugins/neorg.lua` (or `$XDG_CONFIG_HOME/nvim/...`). Targeted
Neorg setup keeps an existing plugin configuration and installs missing plugins
without updating the rest of your editor. It expects the notes workspace at
`~/neorg` and the journal under `journal/`. Custom workspace paths or custom
journal date formats need corresponding changes to mak before use.

In Neovim, use `:Neorg journal today` or `:Neorg journal tomorrow`. Tomorrow's
entry already has tomorrow's date, so it becomes today's entry automatically;
files do not have to move. See [Neorg's journal documentation](https://github.com/nvim-neorg/neorg/wiki/Journal).

Setup installs one cron job: **09:17, Monday–Friday**, in the computer's local
timezone. It runs `mak neorg remind`, which checks today's nested
`journal/YYYY/MM/DD.norg` or flat `journal/YYYY-MM-DD.norg`. A journal containing
notes, headings or todos triggers a notification; whitespace and document metadata
alone do not. This MVP reminds for any journal content, including completed todos.
The computer must be awake and the user logged into a graphical session at that
time; cron does not catch up missed reminders. Allow terminal-notifier notifications
in macOS settings. Timers and reminders use the Crystal sound and optionally
`~/Pictures/tomato.png` when present.

```sh
mak neorg remind               # check manually; silent if there are no notes
mak neorg reminder             # install/refresh the cron entry; safe to repeat
mak neorg reminder --remove    # remove only the Neorg entry
```

Full dev uninstall removes this reminder. Notes and iCloud backups are retained.
Other cron jobs, including mak meetings, are preserved.

```sh
mak sync                       # merge newer files in both directions
mak backup                     # snapshot local notes, then copy newer files to iCloud
mak doctor                     # also checks local iCloud Drive availability
```

Enable **System Settings → Apple Account → iCloud → Drive → Sync this Mac**.
mak uses the iCloud Drive folder already managed by macOS. It does not ask for
cloud credentials or require an API project, browser login, or a separate sync tool.
If that folder is missing or inaccessible, the command stops with setup guidance
rather than creating an ordinary local folder that looks like iCloud.
See [Apple's iCloud Drive setup](https://support.apple.com/en-gb/guide/icloud/mm203b05aec8/icloud).

On first sync or backup, mak creates the hidden `.MAK` directory at the iCloud
Drive root; later runs reuse it:

```text
iCloud Drive/.MAK/
  neorg/                         # all notes and attachments from ~/neorg
  neorg-config/neorg.lua          # only the Neorg plugin config
  backups/<UTC timestamp>/       # complete local snapshots from mak backup
  history/<UTC timestamp>/       # versions replaced by either command
```

On macOS this root normally lives at
`~/Library/Mobile Documents/com~apple~CloudDocs/.MAK`. In Finder, open iCloud Drive
and press **Command–Shift–.** to show hidden folders. mak automatically requests
downloads for macOS dataless files and waits up to two minutes per file before
transferring. If a download fails or times out, check your connection and retry.
Choose **Keep Downloaded** on `.MAK` to prevent macOS from evicting its local
copies. Legacy `.icloud` placeholders still require downloading in Finder before
transferring. See [Apple's download instructions](https://support.apple.com/en-gb/guide/mac-help/-mchl1a02d711/mac).

`mak sync` merges files between `~/neorg` and `.MAK/neorg`, and between the Neorg
plugin config and `.MAK/neorg-config/neorg.lua`. Newer modification times win;
SHA-256 checksums detect differing content even when file size and dates match.
When timestamps tie but contents differ, `sync` keeps the iCloud version and
saves the local version in history. Existing dotfile config symlinks are retained.
Unrelated editor configuration, credentials, and other iCloud folders are untouched.

`mak backup` first creates a complete dated snapshot of your local notes and
Neorg config, including local versions older than those in iCloud. A completed
snapshot contains `manifest.json`. It then copies newer local files to the current
`.MAK/neorg` and `.MAK/neorg-config` folders. It never pulls files or replaces a
newer iCloud version; ties prefer the local version and retain the old iCloud
version in history. Snapshots can be restored by copying their `neorg` and
`neorg-config/neorg.lua` contents back to your local notes and plugin config.

Both commands retain replaced versions under `.MAK/history/<UTC timestamp>/`
and `$XDG_STATE_HOME/mak/neorg/history/` (default `~/.local/state/mak/neorg/history`).
The local last-transfer record includes a UTC date and explicitly records
`cloud_upload_confirmed: false`. File copies are staged, checked, and replaced
atomically. Symlinks inside notes, overlapping storage paths, file/directory
collisions, and files edited during a transfer stop the command before the affected
file is replaced. Save your Neovim buffers before transferring.

**Deletions are not propagated.** Removing a note from one side leaves it on the
other; the next sync restores it. Delete from both current folders intentionally
to remove a note. Dated snapshots and history remain available.

**macOS manages cloud delivery.** A successful command means the local files were
copied into the iCloud Drive folder. It cannot confirm that Apple has finished
uploading them or that another Mac has received them. `mak doctor` checks local
folder availability and permissions without creating files; it does not validate
Apple Account sign-in or server-side upload completion. Use Finder's iCloud status
and wait for transfers to finish before switching machines. Run mak transfers
sequentially across Macs; there is no distributed lock or line-by-line merge.
See [Apple's file status guide](https://support.apple.com/en-gb/guide/mac-help/mchlc994344b/mac).

On a new Mac, sign into the same Apple Account, enable iCloud Drive, then install
mak and run `mak sync`. It restores notes and Neorg
configuration without a separate cloud connection. An existing config participates
in the timestamp merge, so sync before customizing it on the new Mac. Run
`mak setup neorg` afterward to install the plugin or schedule its reminder.

The old rclone package is no longer installed or required. Existing installation
records remain readable; `mak setup dev --remove rclone` can remove a legacy copy
owned by mak. Existing cloud credentials are left alone and are not used by these
commands.

Transfer tests use temporary local folders, require no cloud account, and run as
part of `go test ./internal/productivity`.

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
