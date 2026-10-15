import { Check, FolderTree, LockKeyhole, Terminal, X } from "lucide-react";
import { useState } from "react";
import { ErrorBox, Modal } from "../../components/ui";
import { message, rpc } from "../../services/api";
import type { Bootstrap, Settings } from "../../types";
export function SettingsDialog({
  boot,
  onClose,
  refresh,
}: {
  boot: Bootstrap;
  onClose: () => void;
  refresh: () => Promise<void>;
}) {
  const [settings, setSettings] = useState(boot.settings);
  const [token, setToken] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const field = (key: keyof Settings, value: string) => {
    setSettings((s) => ({ ...s, [key]: value }));
    setSaved(false);
  };
  return (
    <Modal title="Settings & integrations" wide onClose={onClose}>
      <form
        className="settings-form form-body"
        onSubmit={(e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          void rpc<Settings>("settings.save", { settings, token })
            .then(async (s) => {
              setSettings(s);
              setToken("");
              setSaved(true);
              await refresh();
            })
            .catch((e) => setError(message(e)))
            .finally(() => setBusy(false));
        }}
      >
        <section>
          <h3>
            <FolderTree size={16} />
            Local workspace
          </h3>
          <label>
            Worktree root
            <input
              value={settings.worktreeRoot}
              onChange={(e) => field("worktreeRoot", e.target.value)}
              required
            />
          </label>
          <p className="hint">
            New worktrees get a ticket folder with a unique suffix. Existing
            worktrees stay in place.
          </p>
        </section>
        <section>
          <h3>
            <Terminal size={16} />
            Coding agents
          </h3>
          <p className="muted">
            Authenticate in each provider’s CLI. CommitDeck never reads their
            credentials.
          </p>
          <div className="provider-statuses">
            {boot.providers.map((p) => (
              <div key={p.id}>
                <span className={`provider-symbol provider-${p.id}`}>
                  {p.id === "claude" ? "✳" : "◈"}
                </span>
                <strong>{p.name}</strong>
                <span className={p.available ? "green" : "muted"}>
                  {p.available ? <Check size={14} /> : <X size={14} />}
                  {p.available ? "Installed" : "Not found"}
                </span>
              </div>
            ))}
          </div>
          <button
            type="button"
            onClick={() => void refresh().catch((e) => setError(message(e)))}
          >
            Recheck installed CLIs
          </button>
        </section>
        <section>
          <h3>
            <LockKeyhole size={16} />
            Jira Cloud
          </h3>
          <div className="form-grid">
            <label>
              Base URL
              <input
                type="url"
                placeholder="https://your-team.atlassian.net"
                value={settings.jiraBaseUrl}
                onChange={(e) => field("jiraBaseUrl", e.target.value)}
              />
            </label>
            <label>
              Email
              <input
                type="email"
                placeholder="you@company.com"
                value={settings.jiraEmail}
                onChange={(e) => field("jiraEmail", e.target.value)}
              />
            </label>
            <label>
              API token
              <input
                type="password"
                autoComplete="new-password"
                value={token}
                placeholder={
                  settings.jiraTokenSet
                    ? "Token stored securely · leave blank to keep"
                    : "Stored in OS credential storage"
                }
                onChange={(e) => {
                  setToken(e.target.value);
                  setSaved(false);
                }}
              />
            </label>
            <label>
              Default project
              <input
                placeholder="PROJ"
                value={settings.jiraProject}
                onChange={(e) =>
                  field("jiraProject", e.target.value.toUpperCase())
                }
              />
            </label>
          </div>
          <p className="hint">
            <LockKeyhole size={12} />
            Tokens are saved in macOS Keychain, Windows Credential Manager, or
            Linux Secret Service.
          </p>
          {settings.jiraTokenSet && (
            <button
              type="button"
              className="danger ghost"
              onClick={() =>
                void rpc("settings.clearToken")
                  .then(async () => {
                    setSettings((s) => ({ ...s, jiraTokenSet: false }));
                    await refresh();
                  })
                  .catch((e) => setError(message(e)))
              }
            >
              Remove stored Jira token
            </button>
          )}
        </section>
        <section>
          <h3>GitHub</h3>
          <p className="muted">
            Uses the GitHub CLI and the repository’s origin remote.
          </p>
          <code className="setup-command">gh auth login</code>
        </section>
        <section>
          <h3>Local data</h3>
          <code className="data-path">{boot.dataDir}/commitdeck.db</code>
          <p className="hint">
            SQLite stores ticket context and session history. Back up this
            directory while CommitDeck is closed.
          </p>
        </section>
        {error && <ErrorBox error={error} />}
        <div className="form-actions">
          {saved && (
            <span className="green saved-confirm">
              <Check size={14} />
              Settings saved
            </span>
          )}
          <button type="button" onClick={onClose}>
            Close
          </button>
          <button className="primary" disabled={busy}>
            {busy ? "Saving…" : "Save settings"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
