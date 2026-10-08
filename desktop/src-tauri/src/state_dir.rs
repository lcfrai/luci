//! Preserve an existing desktop workspace when upgrading the managed sidecar.
use std::{
    fs, io,
    path::{Path, PathBuf},
};

fn invalid(message: &str) -> io::Error {
    io::Error::new(io::ErrorKind::InvalidData, message)
}

/// An explicit state directory is authoritative. Resolve defaults lazily so
/// isolated launches never inspect a user's current or legacy workspace, even
/// when the explicit path is invalid. Selection itself creates no files.
pub fn select_configured(
    explicit: Option<&Path>,
    defaults: impl FnOnce() -> io::Result<(PathBuf, Option<PathBuf>)>,
) -> io::Result<PathBuf> {
    if let Some(path) = explicit {
        if !path.is_absolute()
            || path.parent().is_none()
            || path.components().any(|part| matches!(part, std::path::Component::ParentDir))
        {
            return Err(invalid("LUCI_STATE_DIR must be an absolute folder, not a drive root or a path containing '..'"));
        }
        return select(path, None);
    }
    let (current, legacy) = defaults()?;
    select(&current, legacy.as_deref())
}

fn linked(metadata: &fs::Metadata) -> bool {
    if metadata.file_type().is_symlink() {
        return true;
    }
    #[cfg(windows)]
    {
        use std::os::windows::fs::MetadataExt;
        // Junctions are reparse points even when is_symlink() is false.
        if metadata.file_attributes() & 0x400 != 0 {
            return true;
        }
    }
    false
}

fn regular_marker(path: &Path) -> io::Result<Option<fs::Metadata>> {
    for ancestor in path.ancestors() {
        match fs::symlink_metadata(ancestor) {
            Ok(metadata) if linked(&metadata) => {
                return Err(invalid("Luci state cannot use a linked file or folder"))
            }
            Ok(_) => {}
            Err(error) if error.kind() == io::ErrorKind::NotFound => {}
            Err(error) => return Err(error),
        }
    }
    match fs::symlink_metadata(path) {
        Ok(metadata) if metadata.is_file() => Ok(Some(metadata)),
        Ok(_) => Err(invalid("Luci state marker must be a regular file")),
        Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(None),
        Err(error) => Err(error),
    }
}

/// Current accounts/state always win. Only a valid, unlinked legacy workspace
/// is reused; nothing is copied, overwritten, deleted, or created here.
pub fn select(current: &Path, legacy: Option<&Path>) -> io::Result<PathBuf> {
    if !current.is_absolute() {
        return Err(invalid("Luci state folder must be absolute"));
    }
    for marker in [
        "control-plane/identity.json",
        "control-plane/bootstrap-token",
        "sessions/luci.json",
        "sessions/sessions.json",
        "instance/instance-id",
    ] {
        if regular_marker(&current.join(marker))?.is_some() {
            return Ok(current.to_owned());
        }
    }
    let Some(legacy) = legacy else {
        return Ok(current.to_owned());
    };
    if !legacy.is_absolute() {
        return Err(invalid("Legacy Luci state folder must be absolute"));
    }
    let path = legacy.join("sessions/luci.json");
    let Some(metadata) = regular_marker(&path)? else {
        return Ok(current.to_owned());
    };
    if metadata.len() == 0 || metadata.len() > 64 * 1024 {
        return Err(invalid("Legacy Luci configuration is empty or too large"));
    }
    let value: serde_json::Value = serde_json::from_slice(&fs::read(path)?)
        .map_err(|_| invalid("Legacy Luci configuration is not valid JSON"))?;
    let projects = value.get("projectsDir").and_then(|value| value.as_str());
    let mode = value.get("mode").and_then(|value| value.as_str());
    if !projects.is_some_and(|path| Path::new(path).is_absolute())
        || !matches!(mode, Some("local" | "cloud"))
    {
        return Err(invalid(
            "Legacy Luci configuration has no valid workspace folder or mode",
        ));
    }
    Ok(legacy.to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;

    struct Fixture(PathBuf);
    impl Fixture {
        fn new() -> Self {
            let root =
                std::env::temp_dir().join(format!("luci-state-selection-{}", uuid::Uuid::new_v4()));
            fs::create_dir(&root).unwrap();
            Self(root)
        }
        fn current(&self) -> PathBuf {
            self.0.join("current")
        }
        fn legacy(&self) -> PathBuf {
            self.0.join("legacy")
        }
        fn write(&self, root: &Path, file: &str, data: &[u8]) {
            fs::create_dir_all(root.join(file).parent().unwrap()).unwrap();
            fs::write(root.join(file), data).unwrap();
        }
        fn old_config(&self) {
            self.write(
                &self.legacy(),
                "sessions/luci.json",
                serde_json::to_string(&serde_json::json!({
                    "projectsDir": self.0.join("projects"), "mode": "local"
                }))
                .unwrap()
                .as_bytes(),
            );
        }
    }
    impl Drop for Fixture {
        fn drop(&mut self) {
            // Only the uniquely created direct child of the actual temp root.
            if self.0.parent().and_then(|path| path.canonicalize().ok())
                == std::env::temp_dir().canonicalize().ok()
                && self
                    .0
                    .file_name()
                    .unwrap()
                    .to_string_lossy()
                    .starts_with("luci-state-selection-")
            {
                let _ = fs::remove_dir_all(&self.0);
            }
        }
    }

    #[test]
    fn state_selection_reuses_legacy_without_copying_or_changing_bytes() {
        let f = Fixture::new();
        f.old_config();
        let before = fs::read(f.legacy().join("sessions/luci.json")).unwrap();
        assert_eq!(select(&f.current(), Some(&f.legacy())).unwrap(), f.legacy());
        assert_eq!(
            fs::read(f.legacy().join("sessions/luci.json")).unwrap(),
            before
        );
        assert!(!f.current().exists());
    }

    #[test]
    fn state_selection_current_accounts_and_config_take_priority() {
        for marker in [
            "control-plane/identity.json",
            "control-plane/bootstrap-token",
            "sessions/luci.json",
            "sessions/sessions.json",
            "instance/instance-id",
        ] {
            let f = Fixture::new();
            f.old_config();
            f.write(&f.current(), marker, b"existing current state");
            assert_eq!(
                select(&f.current(), Some(&f.legacy())).unwrap(),
                f.current()
            );
        }
    }

    #[test]
    fn state_selection_fresh_install_does_not_create_state() {
        let f = Fixture::new();
        assert_eq!(
            select(&f.current(), Some(&f.legacy())).unwrap(),
            f.current()
        );
        assert_eq!(select(&f.current(), None).unwrap(), f.current());
        assert!(!f.current().exists());
        assert!(!f.legacy().exists());
    }

    #[test]
    fn explicit_state_never_resolves_defaults_or_changes_existing_workspaces() {
        let f = Fixture::new();
        f.old_config();
        f.write(&f.current(), "control-plane/identity.json", b"existing account");
        let legacy_before = fs::read(f.legacy().join("sessions/luci.json")).unwrap();
        let explicit = f.0.join("isolated");
        assert_eq!(
            select_configured(Some(&explicit), || panic!("defaults must not be consulted")).unwrap(),
            explicit
        );
        assert!(!explicit.exists());
        assert_eq!(fs::read(f.legacy().join("sessions/luci.json")).unwrap(), legacy_before);
        assert_eq!(fs::read(f.current().join("control-plane/identity.json")).unwrap(), b"existing account");
        f.write(&explicit, "control-plane/bootstrap-token", b"isolated token");
        assert_eq!(
            select_configured(Some(&explicit), || panic!("resume must not consult defaults")).unwrap(),
            explicit
        );
        assert_eq!(fs::read(explicit.join("control-plane/bootstrap-token")).unwrap(), b"isolated token");
    }

    #[test]
    fn invalid_explicit_state_fails_without_default_fallback() {
        let f = Fixture::new();
        let drive_root = f.0.ancestors().last().unwrap();
        for path in [PathBuf::new(), PathBuf::from("relative"), drive_root.to_owned(), f.0.join("missing/../escape")] {
            assert!(select_configured(Some(&path), || panic!("invalid override must not fall back")).is_err());
        }
        let file = f.0.join("not-a-directory");
        fs::write(&file, b"preserve").unwrap();
        assert!(select_configured(Some(&file), || panic!("invalid override must not fall back")).is_err());
        assert_eq!(fs::read(file).unwrap(), b"preserve");
    }

    #[test]
    fn absent_override_keeps_default_legacy_selection() {
        let f = Fixture::new();
        f.old_config();
        assert_eq!(select_configured(None, || Ok((f.current(), Some(f.legacy())))).unwrap(), f.legacy());
        f.write(&f.current(), "control-plane/identity.json", b"current account");
        assert_eq!(select_configured(None, || Ok((f.current(), Some(f.legacy())))).unwrap(), f.current());
    }

    #[test]
    fn state_selection_invalid_legacy_fails_without_overwrite() {
        for data in [
            b"not JSON".as_slice(),
            br#"{"projectsDir":"relative","mode":"local"}"#,
            b"",
        ] {
            let f = Fixture::new();
            f.write(&f.legacy(), "sessions/luci.json", data);
            assert!(select(&f.current(), Some(&f.legacy())).is_err());
            assert_eq!(
                fs::read(f.legacy().join("sessions/luci.json")).unwrap(),
                data
            );
            assert!(!f.current().exists());
        }
    }

    #[test]
    fn state_selection_rejects_linked_legacy_and_current_roots() {
        let f = Fixture::new();
        f.old_config();
        let alias = f.0.join("linked");
        #[cfg(unix)]
        std::os::unix::fs::symlink(f.legacy(), &alias).unwrap();
        #[cfg(windows)]
        {
            // Junction creation requires no developer mode or symlink privilege.
            let output = std::process::Command::new("cmd")
                .args(["/c", "mklink", "/J"])
                .arg(&alias)
                .arg(f.legacy())
                .output()
                .unwrap();
            assert!(
                output.status.success(),
                "junction fixture could not be created"
            );
        }
        assert!(select(&f.current(), Some(&alias)).is_err());
        assert!(select(&alias, None).is_err());
        assert!(select_configured(Some(&alias), || panic!("linked override must not fall back")).is_err());
        // Remove just the link before Fixture drops its known real directory.
        #[cfg(windows)]
        fs::remove_dir(alias).unwrap();
        #[cfg(unix)]
        fs::remove_file(alias).unwrap();
    }

    #[test]
    fn explicit_state_rejects_linked_marker_parent_without_default_discovery() {
        let f = Fixture::new();
        f.old_config();
        let before = fs::read(f.legacy().join("sessions/luci.json")).unwrap();
        let isolated = f.0.join("isolated");
        fs::create_dir(&isolated).unwrap();
        let link = isolated.join("control-plane");
        #[cfg(unix)]
        std::os::unix::fs::symlink(f.legacy().join("sessions"), &link).unwrap();
        #[cfg(windows)]
        {
            let output = std::process::Command::new("cmd")
                .args(["/c", "mklink", "/J"])
                .arg(&link).arg(f.legacy().join("sessions"))
                .output().unwrap();
            assert!(output.status.success(), "junction fixture could not be created");
        }
        assert!(select_configured(Some(&isolated), || panic!("linked override must not inspect defaults")).is_err());
        assert_eq!(fs::read(f.legacy().join("sessions/luci.json")).unwrap(), before);
        #[cfg(windows)]
        fs::remove_dir(link).unwrap();
        #[cfg(unix)]
        fs::remove_file(link).unwrap();
    }
}
