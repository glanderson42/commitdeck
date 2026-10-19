# Community Edition release QA

Candidate: `v0.1.0-beta.1`. This checklist records required verification, not completed installation tests. CommitDeck has no account, billing, telemetry, paid gates, or cloud requirement. Optional providers and integrations require their own credentials and connectivity.

## Automated gates

- Run `npm ci`, `npm test`, `npm run test:release`, `npm run typecheck`, `npm run build`.
- With Go 1.27.2, run `go test ./...`, `go test -race ./...`, `go vet ./...`.
- Build the sidecar, then run `npm run test:smoke`, `npm run test:phase3`, `npm run test:dev` (port 1420 free).
- Run `npm run check:desktop` and a native desktop build with locked Cargo dependencies.
- Run `npm audit --audit-level=high` and `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`.
- CI runs Go/race/frontend/IPC tests on Windows, Linux and macOS. Terminal tests use ConPTY on Windows and a PTY on Unix; fixtures do not use real provider accounts.
- Installer builds cover macOS ARM64 and Intel, Windows x64 NSIS, and Linux x64 Debian. Compilation and a passing terminal unit test do not prove a native installer launches correctly.

## Native installation matrix

Record OS/version, architecture, commit, installer SHA-256, commands, and results for each row. Do not check a platform complete based on another platform's build.

| Platform            | Installer   | Required native evidence                                                   |
| ------------------- | ----------- | -------------------------------------------------------------------------- |
| macOS Apple Silicon | aarch64 DMG | Install, launch, sidecar, shell, restart, uninstall                        |
| macOS Intel         | x64 DMG     | Same on Intel hardware/VM                                                  |
| Windows x64         | NSIS EXE    | Standard-user install, WebView2, ConPTY, restart, uninstall                |
| Debian/Ubuntu x64   | DEB         | Package dependencies, GUI session, PTY, Secret Service, restart, uninstall |

### All platforms: disposable data only

1. Verify `SHA256SUMS` from the approved release. Retain the previous installer and a closed-app data backup.
2. Launch with a new `COMMITDECK_DATA_DIR`; never use real user data or repositories for QA. Create a temporary Git repo with one empty commit.
3. Create two tickets; attach that repo and create separate worktrees. Add/reorder/complete tasks and Markdown notes. Archive/restore one ticket.
4. Open two terminals. Verify working directory, ANSI output, interactive input, resize, Ctrl+C and shell exit. Start a disposable long-running command and close its terminal; verify that process exits and an unrelated shell remains alive.
5. Switch tickets/tabs, close/reopen the app, and verify notes, tasks, session metadata, layout and stopped terminal tabs. Explicitly restart a stopped tab.
6. Attempt a second app against the same data directory; it must refuse the database lock. Quit the first and retry successfully.
7. Test no-network mode: notes, TODOs, Git and terminals still work; integration failures remain local to their panels.
8. Use only fake agent/remote fixtures for destructive/remote actions. Real provider approval channels and authenticated Jira/GitHub writes require a separate, explicitly authorized test account exercise; never count fixture results as live verification.
9. Uninstall/reinstall and verify application data is preserved. Delete disposable QA data only after checking the path.

### macOS

```sh
hdiutil verify CommitDeck_*.dmg
hdiutil attach -readonly -nobrowse CommitDeck_*_aarch64.dmg
codesign --verify --deep --strict /Volumes/CommitDeck/CommitDeck.app
file /Volumes/CommitDeck/CommitDeck.app/Contents/MacOS/commitdeck*
```

Use the actual mounted volume name. Confirm the sidecar is inside the app and has the correct architecture. Drag to Applications and launch from Finder. For a Developer ID build also run `spctl --assess --type execute --verbose=4` and `xcrun stapler validate` on the app. An ad-hoc signature is not Developer ID trust or notarization. Do not disable Gatekeeper globally.

### Windows

PowerShell: `Get-FileHash .\CommitDeck*.exe -Algorithm SHA256` and `Get-AuthenticodeSignature .\CommitDeck*.exe`. Install as a standard user; verify WebView2 is present and the installed application directory contains `commitdeck-core.exe`. Open a ticket terminal, run `cd`, `echo COMMITDECK_OK`, an interactive command, then Ctrl+C and resize. Inspect Task Manager after quitting: no owned core/agent/ConPTY shell processes should remain. Confirm an unrelated terminal survives. Check paths with spaces and non-ASCII characters.

### Debian/Ubuntu

```sh
sha256sum -c SHA256SUMS
dpkg-deb --info ./CommitDeck*.deb
dpkg-deb --contents ./CommitDeck*.deb
sudo apt install ./CommitDeck*.deb
```

Confirm the package contains both desktop and core binaries and its declared dependencies resolve. Launch from the desktop menu in a GUI session with Secret Service unlocked. Verify terminal `pwd`, `stty size`, Ctrl+C and cleanup. Headless Docker PTY tests do not verify WebKitGTK, desktop launch, package installation, or keyring UI.

## Backup and recovery

Migration 001→002→003 preserves notes, TODO ordering, history, archives and session IDs. Each pending upgrade of an existing database first creates `commitdeck.db.pre-migration-*.db` beside it via SQLite `VACUUM INTO`. This is a consistent snapshot including WAL transactions. Backup failure stops the upgrade; each migration runs transactionally. Completed earlier migrations remain if a later migration fails. Repeated startup at the current schema creates no extra migration backup.

Backups are private files, not encrypted archives. They may contain historical data predating secret redaction. They have no automatic retention/deletion policy. Store them on encrypted local storage, restrict access, and review retention manually.

For recovery: quit CommitDeck and confirm its sidecar/terminals have exited. Copy the **entire current data directory** to a separate recovery location, including WAL/SHM files; retain it unchanged. Restore a chosen pre-migration `.db` into a **new empty data directory** as `commitdeck.db` (do not mix old WAL/SHM files with it). Launch a compatible app with `COMMITDECK_DATA_DIR` pointing there. It will migrate again if needed. Verify notes/tasks/history before adopting that directory. Jira credentials remain in OS storage and are not in a database backup. Never restore over a running app or run an older binary against a newer schema.

## Signing and notarization

PR checks and development builds need no signing secrets. The Release workflow accepts optional macOS `APPLE_CERTIFICATE` (base64 P12), `APPLE_CERTIFICATE_PASSWORD`, `APPLE_SIGNING_IDENTITY`, `APPLE_ID`, app-specific `APPLE_PASSWORD`, and `APPLE_TEAM_ID`. Use a Developer ID Application certificate; supply the complete identity and notarization configuration together. Without these, builds are ad-hoc and unnotarized. Follow [Tauri macOS signing](https://v2.tauri.app/distribute/sign/macos/).

Windows releases optionally import `WINDOWS_PFX` (base64 PFX) with `WINDOWS_PFX_PASSWORD` into the ephemeral runner's current-user certificate store, create a temporary Tauri signing config with SHA-256 and timestamping, then remove the imported certificate and PFX. Supply a valid exportable code-signing certificate; hardware-backed EV/cloud signing needs a separately reviewed sign-command integration. For local builds, copy `src-tauri/tauri.windows-signing.conf.example.json` to a private temporary file, supply your installed certificate's thumbprint, and pass `--config <path>` to `npm run desktop:build --`. Never commit a PFX, password or private key. Follow [Tauri Windows signing](https://v2.tauri.app/distribute/sign/windows/).

No signing credentials were supplied for this hardening task. Signed distribution remains unverified. The application has no updater dependency/configuration or signed feed; updates are manual installer replacements after backup. Do not add an updater endpoint or public key without key ownership, signed assets, feed verification and rollback testing.

## Publication checklist (requires explicit maintainer approval)

- [ ] Review this branch and all CI/runtime evidence; approve merge separately.
- [ ] On `main`, wait for the exact intended commit's successful **push** CI, including dependency audit. A PR merge-ref run is insufficient.
- [ ] Verify `package.json`, npm lockfile root, Cargo manifest/lock and Tauri config all say `0.1.0-beta.1`.
- [ ] Complete the native matrix above; record unresolved limitations prominently in beta notes.
- [ ] Supply and validate signing/notarization credentials, or explicitly approve unsigned beta distribution with warnings.
- [ ] Approve creation/push of `v0.1.0-beta.1` at that tested commit. The workflow rejects commits outside `main` or without successful exact-commit main CI.
- [ ] Inspect the generated draft: four nonempty installers, correct architecture/bundled sidecars, `SHA256SUMS`, prerelease flag, notes, backups and known limitations.
- [ ] Explicitly approve publication. Manual dispatch `publish=true` can publish a newly prepared draft; tag pushes default to draft only.
- [ ] Do not replace an existing published release or installer. A partial draft with existing assets requires manual inspection/removal of that **unpublished draft** before retrying; the workflow does not clobber assets.
- [ ] Stable v0.1.0: resolve the existing initial-commit `v0.1.0` tag under explicit approval. Never silently move it. Choose a new stable version if reusing that tag is inappropriate. Finish native install/uninstall, signing and restore testing before stable approval.

### Rollback

Stop publication on any checksum, architecture, migration, signing or launch failure. For an unpublished draft, retain logs and delete/recreate it only after approval. For a published release, never replace assets or move its tag: publish a corrective version and document the affected version; mark/retract the release only with approval. Users should retain backups and the prior installer. An application downgrade requires restoring its compatible database backup in a separate directory; a newer database is not assumed downgrade-compatible.
