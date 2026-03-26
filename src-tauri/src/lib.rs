use serde_json::Value;
use std::collections::HashMap;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{mpsc, Mutex};
use tauri::{AppHandle, Manager};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;

/// Shared state for the persistent sidecar process.
struct SidecarState {
    child: Mutex<Option<CommandChild>>,
    pending: Mutex<HashMap<u64, mpsc::Sender<Result<Value, String>>>>,
    next_id: AtomicU64,
}

impl SidecarState {
    fn new() -> Self {
        Self {
            child: Mutex::new(None),
            pending: Mutex::new(HashMap::new()),
            next_id: AtomicU64::new(1),
        }
    }

    fn send_rpc(&self, method: &str, params: Value) -> Result<Value, String> {
        let id = self.next_id.fetch_add(1, Ordering::Relaxed);

        let request = serde_json::json!({
            "id": id,
            "method": method,
            "params": params,
        });

        let line = serde_json::to_string(&request).map_err(|e| e.to_string())?;

        let (tx, rx) = mpsc::channel();

        // Write to child stdin while holding both locks to prevent races.
        {
            let mut child_guard = self.child.lock().unwrap();
            let child = child_guard.as_mut().ok_or("sidecar not running")?;

            // Insert pending AFTER confirming child exists, BEFORE writing.
            self.pending.lock().unwrap().insert(id, tx);

            let mut payload = line.into_bytes();
            payload.push(b'\n');
            if let Err(e) = child.write(&payload) {
                // Clean up the pending entry on write failure.
                self.pending.lock().unwrap().remove(&id);
                return Err(format!("sidecar write failed: {}", e));
            }
        }

        // Wait for response with timeout.
        match rx.recv_timeout(std::time::Duration::from_secs(300)) {
            Ok(result) => result,
            Err(e) => {
                // Clean up stale pending entry on timeout.
                self.pending.lock().unwrap().remove(&id);
                Err(format!("sidecar response timeout: {}", e))
            }
        }
    }
}

fn start_sidecar(app: &AppHandle) -> Result<(), String> {
    let state = app.state::<SidecarState>();

    // Hold child lock during spawn to prevent double-spawn race.
    let mut child_guard = state.child.lock().unwrap();
    if child_guard.is_some() {
        return Ok(()); // Already started by another caller.
    }

    let (rx, child) = app
        .shell()
        .sidecar("apiw-sidecar")
        .map_err(|e| e.to_string())?
        .args(["serve"])
        .spawn()
        .map_err(|e| e.to_string())?;

    *child_guard = Some(child);
    drop(child_guard); // Release lock before spawning background task.

    let app_handle = app.clone();
    tauri::async_runtime::spawn(async move {
        let state = app_handle.state::<SidecarState>();
        let mut rx = rx;
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stdout(line) => {
                    let line_str = String::from_utf8_lossy(&line);
                    let trimmed = line_str.trim();
                    if trimmed.is_empty() {
                        continue;
                    }

                    match serde_json::from_str::<Value>(trimmed) {
                        Ok(value) => {
                            if let Some(id) = value.get("id").and_then(|v| v.as_u64()) {
                                let result = if let Some(err) =
                                    value.get("error").and_then(|v| v.as_str())
                                {
                                    if err.is_empty() {
                                        Ok(value
                                            .get("result")
                                            .cloned()
                                            .unwrap_or(Value::Null))
                                    } else {
                                        Err(err.to_string())
                                    }
                                } else {
                                    Ok(value.get("result").cloned().unwrap_or(Value::Null))
                                };

                                let tx = {
                                    let mut pending = state.pending.lock().unwrap();
                                    pending.remove(&id)
                                };

                                if let Some(tx) = tx {
                                    let _ = tx.send(result);
                                }
                            }
                        }
                        Err(e) => {
                            eprintln!("sidecar stdout parse error: {} — line: {}", e, trimmed);
                        }
                    }
                }
                CommandEvent::Stderr(line) => {
                    let msg = String::from_utf8_lossy(&line);
                    eprintln!("sidecar stderr: {}", msg.trim());
                }
                CommandEvent::Terminated(payload) => {
                    eprintln!("sidecar terminated with code: {:?}", payload.code);
                    // Clear child handle so ensure_sidecar will restart.
                    *state.child.lock().unwrap() = None;
                    // Notify all pending requests.
                    let mut pending = state.pending.lock().unwrap();
                    for (_, tx) in pending.drain() {
                        let _ = tx.send(Err("sidecar process terminated".to_string()));
                    }
                    break;
                }
                _ => {}
            }
        }
    });

    Ok(())
}

fn ensure_sidecar(app: &AppHandle) -> Result<(), String> {
    start_sidecar(app)
}

#[tauri::command]
async fn load_workspace(app: AppHandle, root: String) -> Result<Value, String> {
    ensure_sidecar(&app)?;
    let app2 = app.clone();
    tauri::async_runtime::spawn_blocking(move || {
        let state = app2.state::<SidecarState>();
        state.send_rpc("workspace", serde_json::json!({ "root": root }))
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn run_request_gui(
    app: AppHandle,
    root: String,
    request_path: String,
    env_name: String,
    timeout_ms: u64,
    snapshot: bool,
) -> Result<Value, String> {
    let timeout_ms = timeout_ms.clamp(100, 300_000);
    ensure_sidecar(&app)?;
    let app2 = app.clone();
    tauri::async_runtime::spawn_blocking(move || {
        let state = app2.state::<SidecarState>();
        state.send_rpc(
            "run_request",
            serde_json::json!({
                "root": root,
                "requestPath": request_path,
                "envName": env_name,
                "timeoutMs": timeout_ms,
                "snapshot": snapshot,
            }),
        )
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn run_collection_gui(
    app: AppHandle,
    root: String,
    collection_path: String,
    env_name: String,
    timeout_ms: u64,
    snapshot: bool,
) -> Result<Value, String> {
    let timeout_ms = timeout_ms.clamp(100, 300_000);
    ensure_sidecar(&app)?;
    let app2 = app.clone();
    tauri::async_runtime::spawn_blocking(move || {
        let state = app2.state::<SidecarState>();
        state.send_rpc(
            "run_collection",
            serde_json::json!({
                "root": root,
                "collectionPath": collection_path,
                "envName": env_name,
                "timeoutMs": timeout_ms,
                "snapshot": snapshot,
            }),
        )
    })
    .await
    .map_err(|e| e.to_string())?
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_shell::init())
        .manage(SidecarState::new())
        .invoke_handler(tauri::generate_handler![
            load_workspace,
            run_request_gui,
            run_collection_gui
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
