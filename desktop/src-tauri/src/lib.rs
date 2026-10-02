use std::{
    io::Write,
    net::{TcpListener, TcpStream},
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

#[derive(Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct RuntimeInfo {
    api_url: String,
    startup_error: Option<String>,
}

struct LocalService {
    child: Mutex<Option<CommandChild>>,
    info: RuntimeInfo,
    exited: Arc<AtomicBool>,
}

#[tauri::command]
fn runtime_info(state: tauri::State<'_, LocalService>) -> RuntimeInfo {
    state.info.clone()
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![runtime_info])
        .setup(|app| {
            let override_url = std::env::var("LUCI_API")
                .ok()
                .filter(|value| !value.is_empty());
            let mut info = RuntimeInfo {
                api_url: override_url
                    .clone()
                    .unwrap_or_else(|| "http://127.0.0.1:8730".into()),
                startup_error: None,
            };
            let mut child = None;
            let exited = Arc::new(AtomicBool::new(false));
            if override_url.is_none() {
                let listener = TcpListener::bind("127.0.0.1:0")?;
                let address = listener.local_addr()?.to_string();
                drop(listener);
                info.api_url = format!("http://{address}");
                let runtime = app.path().resource_dir()?.join("runtime");
                let cloud_bundle = app
                    .path()
                    .resource_dir()?
                    .join("cloud")
                    .join("luci-cloud-linux-amd64.tar.gz");
                match app.shell().sidecar("luci-api").and_then(|command| {
                    command
                        .env("LUCI_ADDR", address)
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
            app.manage(LocalService {
                child: Mutex::new(child),
                info,
                exited,
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
                                    let request = format!("POST /v1/setup/shutdown HTTP/1.1\r\nHost: {address}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n");
                                    let _ = stream.write_all(request.as_bytes());
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
