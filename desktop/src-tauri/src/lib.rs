use std::{
    io::Write,
    net::{TcpListener, TcpStream},
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
    time::Duration,
};
use tauri::Manager;
use tauri_plugin_shell::{
    process::{CommandChild, CommandEvent},
    ShellExt,
};
mod local_access;
mod state_dir;

#[derive(Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct RuntimeInfo {
    api_url: String,
    startup_error: Option<String>,
    state_dir: Option<PathBuf>,
}

impl RuntimeInfo {
    fn select_state(
        &mut self,
        current: &std::path::Path,
        legacy: Option<&std::path::Path>,
    ) -> Option<PathBuf> {
        match state_dir::select(current, legacy) {
            Ok(selected) => {
                self.state_dir = Some(selected.clone());
                Some(selected)
            }
            Err(error) => {
                self.startup_error =
                    Some(format!("Luci could not open its saved workspace: {error}"));
                None
            }
        }
    }
}

struct LocalService {
    child: Mutex<Option<CommandChild>>,
    info: RuntimeInfo,
    exited: Arc<AtomicBool>,
    state_dir: Option<PathBuf>,
    lifecycle_token: Option<String>,
    local_access_token: Option<String>,
}

fn owned_access_connection(
    state: &LocalService,
) -> Result<Option<local_access::Connection>, String> {
    if state.state_dir.is_none() || state.local_access_token.is_none() {
        return Ok(None);
    }
    if state.exited.load(Ordering::Acquire)
        || state
            .child
            .lock()
            .map_err(|_| "Local service state unavailable")?
            .is_none()
    {
        return Err("Luci's local service stopped. Restart Luci to restore account access on this computer.".into());
    }
    let Some(token) = &state.local_access_token else {
        return Ok(None);
    };
    let address = state
        .info
        .api_url
        .strip_prefix("http://")
        .and_then(|value| value.parse::<std::net::SocketAddr>().ok())
        .filter(|address| address.ip().is_loopback())
        .ok_or("Account access is available only for Luci's own local service.")?;
    Ok(Some(local_access::Connection {
        address,
        token: token.clone(),
    }))
}

#[tauri::command]
async fn local_access_info(
    state: tauri::State<'_, LocalService>,
) -> Result<local_access::Info, String> {
    let Some(connection) = owned_access_connection(&state)? else {
        return Ok(local_access::Info {
            available: false,
            owner: None,
        });
    };
    tauri::async_runtime::spawn_blocking(move || local_access::info(&connection))
        .await
        .map_err(|_| "Luci could not check account access on this computer.")?
}

#[tauri::command]
async fn create_local_account(
    state: tauri::State<'_, LocalService>,
    username: String,
    display_name: String,
    password: String,
) -> Result<local_access::AuthResult, String> {
    let connection = owned_access_connection(&state)?
        .ok_or("Create an account using an invitation from this workspace's administrator.")?;
    tauri::async_runtime::spawn_blocking(move || {
        local_access::create(&connection, &username, &display_name, &password)
    })
    .await
    .map_err(|_| "Luci could not finish creating your account.")?
}

#[tauri::command]
async fn recover_local_owner(
    state: tauri::State<'_, LocalService>,
    new_password: String,
) -> Result<local_access::AuthResult, String> {
    let connection = owned_access_connection(&state)?
        .ok_or("Contact this workspace's administrator to recover your sign-in.")?;
    tauri::async_runtime::spawn_blocking(move || local_access::recover(&connection, &new_password))
        .await
        .map_err(|_| "Luci could not finish recovering your account.")?
}

#[tauri::command]
fn runtime_info(state: tauri::State<'_, LocalService>) -> RuntimeInfo {
    state.info.clone()
}

#[tauri::command]
fn bootstrap_token(state: tauri::State<'_, LocalService>) -> Result<Option<String>, String> {
    // An external LUCI_API service never grants this WebView access to local token files.
    let Some(root) = &state.state_dir else {
        return Ok(None);
    };
    if state.exited.load(Ordering::Acquire)
        || !state
            .child
            .lock()
            .map_err(|_| "Local service state unavailable")?
            .is_some()
    {
        return Ok(None);
    }
    let path = root.join("control-plane").join("bootstrap-token");
    let metadata = match std::fs::symlink_metadata(&path) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(_) => return Err("The managed service bootstrap token is unavailable".into()),
    };
    if !metadata.is_file() || metadata.file_type().is_symlink() || metadata.len() > 8192 {
        return Err("The managed service bootstrap token has an invalid format".into());
    }
    let canonical_root = root
        .canonicalize()
        .map_err(|_| "Managed state folder unavailable")?;
    let canonical_path = path
        .canonicalize()
        .map_err(|_| "Managed bootstrap path unavailable")?;
    if !canonical_path.starts_with(canonical_root) {
        return Err("Managed bootstrap path is outside its state folder".into());
    }
    let token = std::fs::read_to_string(canonical_path)
        .map_err(|_| "Managed bootstrap token could not be read")?;
    let token = token.trim();
    if token.is_empty() || !token.bytes().all(|byte| byte.is_ascii_graphic()) {
        return Err("The managed service bootstrap token has an invalid format".into());
    }
    Ok(Some(token.to_owned()))
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![
            runtime_info,
            bootstrap_token,
            local_access_info,
            create_local_account,
            recover_local_owner
        ])
        .setup(|app| {
            let override_url = std::env::var("LUCI_API")
                .ok()
                .filter(|value| !value.is_empty());
            let mut info = RuntimeInfo {
                api_url: override_url
                    .clone()
                    .unwrap_or_else(|| "http://127.0.0.1:8730".into()),
                startup_error: None,
                state_dir: None,
            };
            let mut child = None;
            let mut state_dir = None;
            let mut lifecycle_token = None;
            let mut local_access_token = None;
            let exited = Arc::new(AtomicBool::new(false));
            if override_url.is_none() {
                let listener = TcpListener::bind("127.0.0.1:0")?;
                let address = listener.local_addr()?.to_string();
                drop(listener);
                info.api_url = format!("http://{address}");
                let current_state = app.path().app_data_dir()?;
                let legacy_state = if cfg!(windows) {
                    std::env::var_os("LOCALAPPDATA")
                        .map(|root| PathBuf::from(root).join("Luci").join("state"))
                } else {
                    None
                };
                if let Some(managed_state) =
                    info.select_state(&current_state, legacy_state.as_deref())
                {
                    state_dir = Some(managed_state.clone());
                    let shutdown_token = uuid::Uuid::new_v4().to_string();
                    lifecycle_token = Some(shutdown_token.clone());
                    let access_token = format!(
                        "{}{}",
                        uuid::Uuid::new_v4().simple(),
                        uuid::Uuid::new_v4().simple()
                    );
                    local_access_token = Some(access_token.clone());
                    let runtime = app.path().resource_dir()?.join("runtime");
                    let cloud_bundle = app
                        .path()
                        .resource_dir()?
                        .join("cloud")
                        .join("luci-cloud-linux-amd64.tar.gz");
                    match app.shell().sidecar("luci-api").and_then(|command| {
                        command
                            .env("LUCI_ADDR", address)
                            .env("LUCI_STATE_DIR", managed_state)
                            .env("LUCI_MANAGED_SHUTDOWN_TOKEN", shutdown_token)
                            .env("LUCI_LOCAL_ACCESS_TOKEN", access_token)
                            .env("LUCI_RUNTIME_DIR", runtime)
                            .env("LUCI_CLOUD_BUNDLE", cloud_bundle)
                            .spawn()
                    }) {
                        Ok((mut events, process)) => {
                            child = Some(process);
                            let process_exited = Arc::clone(&exited);
                            // Drain output so a verbose service never blocks on a full pipe.
                            tauri::async_runtime::spawn(async move {
                                while let Some(event) = events.recv().await {
                                    if matches!(event, CommandEvent::Terminated(_)) {
                                        process_exited.store(true, Ordering::Release);
                                    }
                                }
                            });
                        }
                        Err(error) => {
                            info.startup_error =
                                Some(format!("Luci could not start its local service: {error}"))
                        }
                    }
                }
            }
            app.manage(LocalService {
                child: Mutex::new(child),
                info,
                exited,
                state_dir,
                lifecycle_token,
                local_access_token,
            });
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("Luci could not start its desktop window");
    app.run(|handle, event| {
        if matches!(event, tauri::RunEvent::Exit) {
            if let Some(service) = handle.try_state::<LocalService>() {
                if let Ok(mut child) = service.child.lock() {
                    if let Some(process) = child.take() {
                        // Ask our own service to stop tunnels and flush state before termination.
                        // An LUCI_API override has no owned child and never reaches this branch.
                        if let Some(address) = service.info.api_url.strip_prefix("http://") {
                            if let Ok(address) = address.parse() {
                                if let Ok(mut stream) = TcpStream::connect_timeout(&address, Duration::from_millis(500)) {
                                    let _ = stream.set_write_timeout(Some(Duration::from_millis(500)));
                                    if let Some(token) = &service.lifecycle_token {
                                        let request = format!("POST /v1/control-plane/shutdown HTTP/1.1\r\nHost: {address}\r\nX-Luci-Lifecycle-Token: {token}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n");
                                        let _ = stream.write_all(request.as_bytes());
                                    }
                                }
                            }
                        }
                        for _ in 0..100 {
                            if service.exited.load(Ordering::Acquire) { break; }
                            std::thread::sleep(Duration::from_millis(50));
                        }
                        if !service.exited.load(Ordering::Acquire) { let _ = process.kill(); }
                    }
                }
            }
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn state_selection_failure_is_visible_without_starting_a_service() {
        let mut info = RuntimeInfo {
            api_url: "http://127.0.0.1:1".into(),
            startup_error: None,
            state_dir: None,
        };
        assert!(info
            .select_state(std::path::Path::new("relative"), None)
            .is_none());
        assert!(info.state_dir.is_none());
        assert!(info
            .startup_error
            .unwrap()
            .contains("could not open its saved workspace"));
    }

    #[test]
    fn account_capability_is_unavailable_without_an_owned_live_child() {
        let mut service = LocalService {
            child: Mutex::new(None),
            info: RuntimeInfo {
                api_url: "http://127.0.0.1:1".into(),
                startup_error: None,
                state_dir: None,
            },
            exited: Arc::new(AtomicBool::new(false)),
            state_dir: Some(PathBuf::from("unused-state")),
            lifecycle_token: None,
            local_access_token: Some("a".repeat(64)),
        };
        assert!(owned_access_connection(&service)
            .err()
            .unwrap()
            .contains("Restart Luci"));
        service.state_dir = None;
        assert!(owned_access_connection(&service).unwrap().is_none());
        // The only serializable runtime metadata must not disclose either
        // native authority or a local account's password/session.
        let metadata = serde_json::to_string(&service.info).unwrap();
        assert!(!metadata.contains("localAccessToken"));
        assert!(!metadata.contains(&"a".repeat(64)));
    }
}
