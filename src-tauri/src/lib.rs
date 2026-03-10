use serde_json::Value;
use tauri::AppHandle;
use tauri_plugin_shell::ShellExt;

#[tauri::command]
async fn load_workspace(app: AppHandle, root: String) -> Result<Value, String> {
    run_sidecar_json(&app, vec!["workspace".into(), "--root".into(), root]).await
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
    let mut args = vec![
        "run-request".into(),
        "--root".into(),
        root,
        "--request".into(),
        request_path,
        "--env".into(),
        env_name,
        "--timeout-ms".into(),
        timeout_ms.to_string(),
    ];

    if snapshot {
        args.push("--snapshot".into());
    }

    run_sidecar_json(&app, args).await
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
    let mut args = vec![
        "run-collection".into(),
        "--root".into(),
        root,
        "--collection".into(),
        collection_path,
        "--env".into(),
        env_name,
        "--timeout-ms".into(),
        timeout_ms.to_string(),
    ];

    if snapshot {
        args.push("--snapshot".into());
    }

    run_sidecar_json(&app, args).await
}

async fn run_sidecar_json(app: &AppHandle, args: Vec<String>) -> Result<Value, String> {
    let output = app
        .shell()
        .sidecar("apiw-sidecar")
        .map_err(|error| error.to_string())?
        .args(args)
        .output()
        .await
        .map_err(|error| error.to_string())?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
        let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
        let message = if !stderr.is_empty() {
            stderr
        } else if !stdout.is_empty() {
            stdout
        } else {
            format!("sidecar exited with code {:?}", output.status.code())
        };
        return Err(message);
    }

    serde_json::from_slice(&output.stdout).map_err(|error| error.to_string())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![
            load_workspace,
            run_request_gui,
            run_collection_gui
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
