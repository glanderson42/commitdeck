# Contributing

Use Node 24, Go 1.27.2, Rust stable and the native Tauri prerequisites in README. Run `npm ci`, `npm run desktop`, and use disposable data via `COMMITDECK_DATA_DIR` for experiments. Authenticate external CLIs yourself only when intentionally testing an integration.

Keep changes focused, preserve normalized provider interfaces and existing user data, and add regression tests for behavioral fixes. SQL changes require additive transactional migrations and recovery tests. Process execution uses explicit arguments. Never commit credentials, backups, provider stores, terminal logs or user worktrees.

Before a PR, run the validation commands in [release QA](docs/release-qa.md). Document the exact platforms and commands exercised. Fixture results are not live-provider or native-install verification. Include migration, security and compatibility implications. Open security issues privately as described in SECURITY.md.

Do not create release tags, publish installers, alter signing settings, or change repository protection without maintainer approval. Community functionality must remain local-first with no paid gates, billing, telemetry or required cloud accounts.
