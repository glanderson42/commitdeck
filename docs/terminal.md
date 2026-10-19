# Integrated terminals

Open **Terminal** or use **Cmd/Ctrl+3**. **Cmd/Ctrl+Shift+T** creates another shell. Each tab starts in the ticket's existing Git worktree; repository-only and archived workspaces cannot start shells. On Unix the shell comes from `SHELL`, falling back to `/bin/sh`, with login-shell startup. Windows uses `COMSPEC` (usually `cmd.exe`) and requires ConPTY support on Windows 10 1809 or newer.

The frontend uses xterm.js and its Fit addon, loaded when a terminal is first opened. ANSI output, shell completion, interactive programs, Ctrl+C, selection, clipboard actions, scrollback, and resize are handled by the emulator and PTY. Keyboard shortcuts from the application do not intercept keystrokes while terminal input has focus. Use the titlebar command button to open the palette from there.

The Go `internal/terminal` manager owns every shell and PTY. Unix uses `creack/pty`; Windows uses `UserExistsError/conpty`. IPC carries explicit create/start/write/resize/rename/delete requests and sequenced base64 output events. A snapshot plus sequence number restores a view without replaying the same output twice. Backend buffers are limited to 256 KiB per running terminal and frontend scrollback to 5,000 lines. There is no terminal-content database or extra shell-history file.

Use **Split terminal** to dock terminals under Agent, Notes, Git, or another workspace pane. Sidebar width, Git file-list width, terminal dock height, selected terminal, split visibility, and per-ticket tab selection are persisted. Double-click a terminal tab or use Rename to name it `tests` or `logs`.

Only selected output is sent through **Send selected output to an agent**. The editable prompt labels that output as untrusted data and limits the selection to 16,000 characters. You choose a new or existing session before sending. The whole buffer is never sent automatically.

Closing a terminal asks for confirmation and terminates its attached shell/processes. Unix cleanup identifies process groups within the owned PTY session, including shell job-control groups; Windows closes the owned pseudoconsole. Deliberately detached daemons that leave that session are outside this ownership boundary. Archive, delete, repository changes, and worktree cleanup are refused while shells or agents are active. Stop/close them first.

After application restart, tab names and working directories remain but processes are **Stopped**. Use **Restart shell** to launch a new process. Scrollback is memory-only and is not recovered after application/backend restart. Overview counts running shells; it does not infer whether each foreground command succeeded. Terminal exit-code notifications and reconnecting to external terminals are not implemented.

The terminal inherits the user's normal environment and login-shell configuration. CommitDeck does not copy native credential-store secrets into it. Commands typed by the developer have ordinary shell access; agent permission rules do not sandbox a developer's terminal.

The PTY lifecycle, worktree CWD, resize, metadata recovery, and cleanup have automated Unix tests and a real IPC smoke check. Windows and Linux sidecars cross-compile; live Windows ConPTY behavior remains a platform verification item.
