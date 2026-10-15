#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use serde_json::{json, Value};
use std::{
    collections::HashMap,
    io::{BufRead, BufReader, Write},
    process::{Child, ChildStdin, Command, Stdio},
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc, Mutex,
    },
    time::Duration,
};
use tauri::{Emitter, Manager};
use tokio::sync::oneshot;

type Pending = Arc<Mutex<HashMap<u64, oneshot::Sender<Result<Value, String>>>>>;
struct Core {
    stdin: Mutex<Option<ChildStdin>>,
    child: Mutex<Child>,
    pending: Pending,
    next: AtomicU64,
}

#[tauri::command]
async fn rpc(core: tauri::State<'_, Core>, method: String, params: Value) -> Result<Value, String> {
    let id = core.next.fetch_add(1, Ordering::Relaxed);
    let (tx, rx) = oneshot::channel();
    core.pending
        .lock()
        .map_err(|e| e.to_string())?
        .insert(id, tx);
    let line = format!("{}\n", json!({"id":id,"method":method,"params":params}));
    let result = core
        .stdin
        .lock()
        .map_err(|e| e.to_string())?
        .as_mut()
        .ok_or("Backend stopped")?
        .write_all(line.as_bytes());
    if let Err(e) = result {
        core.pending.lock().unwrap().remove(&id);
        return Err(e.to_string());
    }
    match tokio::time::timeout(Duration::from_secs(120), rx).await {
        Ok(Ok(v)) => v,
        _ => {
            core.pending.lock().unwrap().remove(&id);
            Err("Backend request timed out or disconnected".into())
        }
    }
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .setup(|app| {
            let exe = std::env::current_exe()?;
            let name = if cfg!(windows) {
                "commitdeck-core.exe"
            } else {
                "commitdeck-core"
            };
            let path = if cfg!(debug_assertions) {
                std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                    .join("../bin")
                    .join(name)
            } else {
                exe.parent()
                    .ok_or("Missing executable directory")?
                    .join(name)
            };
            let mut command = Command::new(path);
            // Finder launches with a minimal PATH. Include conventional CLI install locations.
            let mut paths = std::env::split_paths(&std::env::var_os("PATH").unwrap_or_default())
                .collect::<Vec<_>>();
            if let Some(home) = std::env::var_os(if cfg!(windows) { "USERPROFILE" } else { "HOME" })
            {
                for suffix in [".local/bin", ".cargo/bin", "go/bin"] {
                    paths.push(std::path::PathBuf::from(&home).join(suffix));
                }
            }
            if !cfg!(windows) {
                for p in ["/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin"] {
                    paths.push(p.into());
                }
            }
            command.env("PATH", std::env::join_paths(paths)?);
            let mut child = command
                .stdin(Stdio::piped())
                .stdout(Stdio::piped())
                .stderr(Stdio::inherit())
                .spawn()?;
            let stdin = child.stdin.take().ok_or("Missing backend stdin")?;
            let stdout = child.stdout.take().ok_or("Missing backend stdout")?;
            let pending: Pending = Arc::new(Mutex::new(HashMap::new()));
            let reader_pending = pending.clone();
            let handle = app.handle().clone();
            std::thread::spawn(move || {
                for line in BufReader::new(stdout).lines().map_while(Result::ok) {
                    if let Ok(v) = serde_json::from_str::<Value>(&line) {
                        if let Some(id) = v["id"].as_u64() {
                            if let Some(tx) = reader_pending.lock().unwrap().remove(&id) {
                                let result = if let Some(err) = v["error"].as_str() {
                                    Err(err.into())
                                } else {
                                    Ok(v["result"].clone())
                                };
                                let _ = tx.send(result);
                            }
                        } else if v["event"].is_object() {
                            let _ = handle.emit("core-event", &v["event"]);
                        }
                    }
                }
                for (_, tx) in reader_pending.lock().unwrap().drain() {
                    let _ = tx.send(Err("Backend disconnected. Restart CommitDeck.".into()));
                }
                let _ = handle.emit("core-disconnected", ());
            });
            app.manage(Core {
                stdin: Mutex::new(Some(stdin)),
                child: Mutex::new(child),
                pending,
                next: AtomicU64::new(1),
            });
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![rpc])
        .build(tauri::generate_context!())
        .expect("Unable to start CommitDeck")
        .run(|app, event| {
            if let tauri::RunEvent::Exit = event {
                if let Some(core) = app.try_state::<Core>() {
                    core.stdin.lock().unwrap().take();
                    // EOF lets Go cancel only processes it owns and flush SQLite before exit.
                    let mut child = core.child.lock().unwrap();
                    for _ in 0..80 {
                        if child.try_wait().ok().flatten().is_some() {
                            return;
                        }
                        std::thread::sleep(Duration::from_millis(100));
                    }
                    let _ = child.kill();
                    let _ = child.wait();
                }
            }
        });
}
