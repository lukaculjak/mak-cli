# mak

`mak` is a personal developer CLI for generating validation composables, opening
recurring meetings, and managing encrypted browser-login prefills.

## Install

macOS and Linux releases are available for AMD64 and ARM64:

```sh
curl -fsSL https://raw.githubusercontent.com/lukaculjak/mak-cli/main/scripts/install.sh | bash
```

Run `mak --help` to see all commands.

## Commands

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
