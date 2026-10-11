#[cfg(not(windows))]
compile_error!("Run this controlled kernel-test harness on Windows.");
#[cfg(windows)]
#[path = "../fixed/desktop/src-tauri/src/windows_sidecar.rs"]
mod windows_sidecar;
