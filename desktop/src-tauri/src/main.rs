// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    if std::env::args_os().nth(1).as_deref()
        == Some(std::ffi::OsStr::new("--check-embedded-frontend"))
    {
        match luci_lib::check_embedded_frontend() {
            Ok(count) => println!("Luci embedded frontend verified: {count} local entrypoint assets"),
            Err(error) => {
                eprintln!("Luci embedded frontend verification failed: {error}");
                std::process::exit(1);
            }
        }
        return;
    }
    luci_lib::run()
}
