# Security

CommitDeck Community Edition stores development data locally. Jira credentials use the OS credential store; provider authentication stays with supported external CLIs. There is no telemetry, cloud account or plaintext-token fallback.

Report vulnerabilities privately through GitHub private vulnerability reporting if enabled. Otherwise request a private maintainer contact without posting credentials or exploit details publicly. Include the version, platform, affected boundary and a synthetic reproduction. Never include real tokens, private repository contents, database backups or full terminal output.

Agent events are redacted before storage and streaming; activity metadata and generated context have additional protection. Pattern-based redaction is best effort, not a guarantee for arbitrary/fragmented/encoded secrets, and does not rewrite old history or backups. Avoid putting credentials in prompts, notes, tool arguments or terminal selections. A terminal is a normal developer shell, not a sandbox. Agent permission mediation does not replace OS/provider security.

Use patched toolchains and dependencies, keep backups on encrypted storage, and audit proposed remote/destructive actions. This beta has no automatic updates. Only verified release artifacts should be installed; unsigned builds may trigger OS warnings. See [release QA](docs/release-qa.md) for signing, recovery and known verification limits.
