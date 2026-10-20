#[path = "src/document_runtime.rs"]
#[allow(dead_code)]
mod document_runtime;

fn main() {
    const INPUTS: [&str; 4] = [
        "LUCI_BUILD_DOCUMENT_PROFILE_SHA256",
        "LUCI_BUILD_DOCUMENT_TOOL_DOMAIN_SHA256",
        "LUCI_BUILD_DOCUMENT_IMAGE_MANIFEST_SHA256",
        "LUCI_BUILD_DOCUMENT_BUNDLE_REQUIRED",
    ];
    for name in INPUTS {
        println!("cargo:rerun-if-env-changed={name}");
    }
    let values = INPUTS.map(|name| match std::env::var(name) {
        Ok(value) => value,
        Err(std::env::VarError::NotPresent) => String::new(),
        Err(std::env::VarError::NotUnicode(_)) => panic!("{name} must be UTF-8"),
    });
    let pins = document_runtime::DocumentRuntimePins::validate(
        &values[0], &values[1], &values[2], &values[3],
    ).unwrap_or_else(|error| panic!("Invalid packaged document runtime: {error}"));
    for (name, value) in [
        ("LUCI_BUNDLED_DOCUMENT_PROFILE_SHA256", pins.profile),
        ("LUCI_BUNDLED_DOCUMENT_TOOL_DOMAIN_SHA256", pins.tool_domain),
        ("LUCI_BUNDLED_DOCUMENT_IMAGE_MANIFEST_SHA256", pins.image_manifest),
    ] {
        println!("cargo:rustc-env={name}={value}");
    }
    tauri_build::build()
}
