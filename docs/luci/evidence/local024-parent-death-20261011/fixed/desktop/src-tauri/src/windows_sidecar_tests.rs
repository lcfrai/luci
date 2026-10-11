//! Controlled Windows process tests. These exercise the production launcher,
//! with no API, Docker daemon, WebView, credentials or personal workspace.
//! Run the ordinary tests; `fixture` is an ignored subprocess entry point.
use super::{new_job, spawn, spawn_inner};
use std::{
    ffi::OsString,
    fs, io,
    os::windows::io::{AsRawHandle, FromRawHandle, OwnedHandle},
    path::{Path, PathBuf},
    process::{Command, Stdio},
    sync::{
        atomic::{AtomicBool, AtomicU64, Ordering},
        Arc,
    },
    thread,
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};
use windows_sys::Win32::{
    Foundation::{FILETIME, WAIT_FAILED, WAIT_OBJECT_0, WAIT_TIMEOUT},
    System::{
        JobObjects::AssignProcessToJobObject,
        Threading::{
            GetCurrentProcess, GetProcessTimes, OpenProcess, TerminateProcess,
            WaitForSingleObject, PROCESS_QUERY_LIMITED_INFORMATION, PROCESS_TERMINATE,
        },
    },
};

const FIXTURE: &str = "windows_sidecar::tests::fixture";
const ROLE: &str = "LUCI_TEST_OWNED_SIDECAR_ROLE";
const ROOT: &str = "LUCI_TEST_OWNED_SIDECAR_ROOT";
const READY_LIMIT: Duration = Duration::from_secs(20);
const EXIT_LIMIT: Duration = Duration::from_secs(10);
// A broken test must not leave a fixture alive indefinitely. This exceeds the
// normal bounded observation window; successful tests terminate every fixture.
const FIXTURE_LIFETIME: Duration = Duration::from_secs(60);
const SYNCHRONIZE_ACCESS: u32 = 0x0010_0000;
static NEXT_ROOT: AtomicU64 = AtomicU64::new(0);

struct TestRoot(PathBuf);

impl TestRoot {
    fn new() -> io::Result<Self> {
        let stamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_err(io::Error::other)?
            .as_nanos();
        // Spaces and Unicode also exercise native executable/environment/CWD
        // transport, without depending on a particular user's profile path.
        let path = std::env::temp_dir().join(format!(
            "Luci owned sidecar 世界 {}-{stamp}-{}",
            std::process::id(),
            NEXT_ROOT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path)?;
        Ok(Self(path))
    }
}

impl Drop for TestRoot {
    fn drop(&mut self) {
        if thread::panicking() {
            eprintln!("retained failed owned-sidecar fixture: {}", self.0.display());
            return;
        }
        // All process guards are declared after this guard and drop first.
        // A failed deletion remains a failed-test artifact, never a reason to
        // remove anything beyond this uniquely created fixture directory.
        let _ = fs::remove_dir_all(&self.0);
    }
}

/// A handle opened while the exact fixture is live, retained through checks.
/// No later cleanup reopens a PID that could have been reused.
struct ProcessWitness {
    handle: OwnedHandle,
    pid: u32,
    created: u64,
}

impl ProcessWitness {
    fn open(pid: u32) -> io::Result<Self> {
        let raw = unsafe {
            OpenProcess(
                SYNCHRONIZE_ACCESS | PROCESS_QUERY_LIMITED_INFORMATION | PROCESS_TERMINATE,
                0,
                pid,
            )
        };
        if raw.is_null() {
            return Err(io::Error::last_os_error());
        }
        let handle = unsafe { OwnedHandle::from_raw_handle(raw) };
        let mut created: FILETIME = unsafe { std::mem::zeroed() };
        let mut ended: FILETIME = unsafe { std::mem::zeroed() };
        let mut kernel: FILETIME = unsafe { std::mem::zeroed() };
        let mut user: FILETIME = unsafe { std::mem::zeroed() };
        if unsafe {
            GetProcessTimes(handle.as_raw_handle(), &mut created, &mut ended, &mut kernel, &mut user)
        } == 0
        {
            return Err(io::Error::last_os_error());
        }
        Ok(Self {
            handle,
            pid,
            created: ((created.dwHighDateTime as u64) << 32) | created.dwLowDateTime as u64,
        })
    }

    fn wait(&self, timeout: Duration) -> io::Result<bool> {
        let millis = u32::try_from(timeout.as_millis()).unwrap_or(u32::MAX - 1);
        match unsafe { WaitForSingleObject(self.handle.as_raw_handle(), millis) } {
            WAIT_OBJECT_0 => Ok(true),
            WAIT_TIMEOUT => Ok(false),
            WAIT_FAILED => Err(io::Error::last_os_error()),
            value => Err(io::Error::other(format!("unexpected process wait result {value}"))),
        }
    }

    fn terminate(&self) -> io::Result<()> {
        if !self.wait(Duration::ZERO)?
            && unsafe { TerminateProcess(self.handle.as_raw_handle(), 91) } == 0
        {
            return Err(io::Error::last_os_error());
        }
        if !self.wait(EXIT_LIMIT)? {
            return Err(io::Error::new(io::ErrorKind::TimedOut, "fixture did not terminate"));
        }
        Ok(())
    }

    fn assert_exited(&self, description: &str) {
        assert!(
            self.wait(EXIT_LIMIT).expect("wait for owned fixture"),
            "{description} survived: pid={}, creation={}",
            self.pid,
            self.created
        );
    }
}

impl Drop for ProcessWitness {
    fn drop(&mut self) {
        // Failure cleanup is restricted to this retained, test-owned handle.
        let _ = self.terminate();
    }
}

fn fixture_command(role: &str, root: &Path) -> io::Result<Command> {
    let mut command = Command::new(std::env::current_exe()?);
    command
        .args(["--ignored", "--exact", FIXTURE, "--nocapture", "--test-threads=1"])
        .env(ROLE, role)
        .env(ROOT, root)
        .current_dir(root)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    Ok(command)
}

fn fixture_errors(root: &Path) -> String {
    ["parent", "sidecar", "grandchild", "sibling", "idle", "finish"]
        .iter()
        .filter_map(|role| {
            fs::read_to_string(root.join(format!("{role}.error")))
                .ok()
                .map(|error| format!("{role}: {}", error.chars().take(2048).collect::<String>()))
        })
        .collect::<Vec<_>>()
        .join("; ")
}

fn wait_file(root: &Path, name: &str, deadline: Instant) -> String {
    loop {
        if let Ok(value) = fs::read_to_string(root.join(name)) {
            if !value.is_empty() {
                return value;
            }
        }
        let errors = fixture_errors(root);
        assert!(errors.is_empty(), "fixture failure while awaiting {name}: {errors}");
        assert!(Instant::now() < deadline, "timed out awaiting {name} in {}", root.display());
        thread::sleep(Duration::from_millis(10));
    }
}

fn wait_process(root: &Path, role: &str, deadline: Instant) -> ProcessWitness {
    let value = wait_file(root, &format!("{role}.pid"), deadline);
    let pid = value.trim().parse::<u32>().expect("fixture PID record");
    let process = ProcessWitness::open(pid).expect("open exact live fixture process");
    assert!(!process.wait(Duration::ZERO).expect("inspect fixture"), "{role} already exited");
    process
}

fn publish(root: &Path, name: &str, value: impl AsRef<[u8]>) -> io::Result<()> {
    // Rename prevents observers from reading a partially written PID marker.
    let temporary = root.join(format!("{name}.{}.tmp", std::process::id()));
    fs::write(&temporary, value)?;
    fs::rename(temporary, root.join(name))
}

fn remain_alive() {
    thread::sleep(FIXTURE_LIFETIME);
}

#[test]
fn abrupt_parent_death_at_create_boundary_kills_only_owned_tree() -> io::Result<()> {
    let root = TestRoot::new()?;
    let mut command = fixture_command("parent", &root.0)?;
    let mut child = command.spawn()?;
    let parent = match ProcessWitness::open(child.id()) {
        Ok(witness) => witness,
        Err(error) => {
            let _ = child.kill();
            let _ = child.wait();
            return Err(error);
        }
    };
    // Child's handle can close: the retained witness identifies the process.
    drop(child);
    let deadline = Instant::now() + READY_LIMIT;
    let sibling = wait_process(&root.0, "sibling", deadline);
    let sidecar = wait_process(&root.0, "sidecar", deadline);
    assert_eq!(wait_file(&root.0, "post-create.barrier", deadline), "blocked");
    assert_eq!(wait_file(&root.0, "nested-job.ok", deadline), "assigned");
    let grandchild = wait_process(&root.0, "grandchild", deadline);
    assert_eq!(wait_file(&root.0, "grandchild.ready", deadline), "ready");

    // The launch function has not returned. This does not run the parent's
    // Rust destructors or normal-exit callback, matching abrupt native death.
    assert!(!parent.wait(Duration::ZERO)?);
    parent.terminate()?;
    sidecar.assert_exited("owned sidecar at post-Create boundary");
    grandchild.assert_exited("owned sidecar descendant in nested job");
    assert!(
        !sibling.wait(Duration::from_millis(250))?,
        "ordinary sibling was incorrectly captured by an app-wide job"
    );
    // Independently terminate the test's unrelated sentinel after survival is
    // established. This is not credited as owned-job cleanup.
    sibling.terminate()?;
    Ok(())
}

#[test]
fn successful_create_followed_by_setup_error_terminates_the_child() -> io::Result<()> {
    let root = TestRoot::new()?;
    let exited = Arc::new(AtomicBool::new(false));
    let mut witness = None;
    let result = spawn_inner(fixture_command("idle", &root.0)?, exited, |pid| {
        witness = Some(ProcessWitness::open(pid)?);
        Err(io::Error::other("injected post-create setup failure"))
    });
    match result {
        Ok(child) => {
            let _ = child.kill();
            panic!("post-create setup failure was ignored");
        }
        Err(error) => assert!(error.to_string().contains("injected post-create setup failure")),
    }
    witness.expect("hook ran after a real process was created")
        .assert_exited("child after post-create setup error");
    Ok(())
}

#[test]
fn ordinary_child_exit_updates_the_live_child_flag_without_killing() -> io::Result<()> {
    let root = TestRoot::new()?;
    let exited = Arc::new(AtomicBool::new(false));
    let child = spawn(fixture_command("finish", &root.0)?, Arc::clone(&exited))?;
    let witness = ProcessWitness::open(child.pid())?;
    let deadline = Instant::now() + READY_LIMIT;
    assert_eq!(wait_file(&root.0, "finish.ready", deadline), "ready");
    assert!(!exited.load(Ordering::Acquire), "live fixture was reported exited");
    publish(&root.0, "finish.allowed", "exit")?;
    witness.assert_exited("naturally finishing child");
    while !exited.load(Ordering::Acquire) {
        assert!(Instant::now() < deadline, "waiter did not publish direct child exit");
        thread::sleep(Duration::from_millis(10));
    }
    assert_eq!(fs::read_to_string(root.0.join("finish.completed"))?, "completed");
    // No kill was requested. This checks the process watcher, not the native
    // HTTP graceful-shutdown contract or application state flushing.
    drop(child);
    Ok(())
}

#[test]
fn missing_executable_is_a_startup_error_without_a_fallback_child() -> io::Result<()> {
    let root = TestRoot::new()?;
    let command = Command::new(root.0.join("definitely-missing-owned-sidecar.exe"));
    let exited = Arc::new(AtomicBool::new(false));
    let mut hook_called = false;
    let result = spawn_inner(command, Arc::clone(&exited), |_| {
        hook_called = true;
        Ok(())
    });
    if let Ok(child) = result {
        let _ = child.kill();
        panic!("missing executable unexpectedly launched");
    }
    assert!(!hook_called, "missing executable reached successful process creation");
    assert!(!exited.load(Ordering::Acquire));
    assert_eq!(fs::read_dir(&root.0)?.count(), 0, "startup failure wrote fixture state");
    Ok(())
}

#[test]
fn malformed_environment_refuses_before_child_code_runs() -> io::Result<()> {
    let root = TestRoot::new()?;
    let mut command = fixture_command("idle", &root.0)?;
    command.env("LUCI_TEST_INVALID_VALUE", OsString::from("before\0after"));
    let result = spawn(command, Arc::new(AtomicBool::new(false)));
    if let Ok(child) = result {
        let _ = child.kill();
        panic!("NUL-bearing environment value was accepted or silently truncated");
    }
    assert!(!root.0.join("idle.pid").exists(), "refused launch executed fixture code");
    Ok(())
}

#[test]
#[ignore = "subprocess fixture; launched only by the bounded controlled tests"]
fn fixture() {
    let Some(role) = std::env::var_os(ROLE) else { return };
    let role = role.to_string_lossy().into_owned();
    let root = PathBuf::from(std::env::var_os(ROOT).expect("fixture root"));
    if let Err(error) = run_fixture(&role, &root) {
        let _ = publish(&root, &format!("{role}.error"), error.to_string());
        panic!("{role} fixture failed: {error}");
    }
}

fn run_fixture(role: &str, root: &Path) -> io::Result<()> {
    if std::env::current_dir()? != root {
        return Err(io::Error::other("fixture CWD did not preserve the configured Unicode path"));
    }
    match role {
        "parent" => {
            let mut sibling = fixture_command("sibling", root)?.spawn()?;
            // Record immediately so the observer can acquire a retained handle
            // even if subsequent launch setup fails.
            if let Err(error) = publish(root, "sibling.pid", sibling.id().to_string()) {
                let _ = sibling.kill();
                let _ = sibling.wait();
                return Err(error);
            }
            let result = spawn_inner(
                fixture_command("sidecar", root)?,
                Arc::new(AtomicBool::new(false)),
                |pid| {
                    publish(root, "sidecar.pid", pid.to_string())?;
                    publish(root, "post-create.barrier", "blocked")?;
                    // The observer terminates this process here, before the
                    // production launcher installs its waiter or returns.
                    remain_alive();
                    Err(io::Error::new(io::ErrorKind::TimedOut, "parent boundary was not terminated"))
                },
            );
            let _ = sibling.kill();
            let _ = sibling.wait();
            match result {
                Ok(child) => {
                    let _ = child.kill();
                    Err(io::Error::other("parent unexpectedly escaped its creation barrier"))
                }
                Err(error) => Err(error),
            }
        }
        "sidecar" => {
            // Mirror the Go API's protectChildren: a fresh non-inheritable
            // kill-on-close job, followed by self-assignment while already in
            // the desktop's outer job. HTTP readiness cannot prove this.
            let nested = new_job()?;
            if unsafe { AssignProcessToJobObject(nested.as_raw_handle(), GetCurrentProcess()) } == 0 {
                return Err(io::Error::last_os_error());
            }
            publish(root, "nested-job.ok", "assigned")?;
            let mut grandchild = fixture_command("grandchild", root)?.spawn()?;
            if let Err(error) = publish(root, "grandchild.pid", grandchild.id().to_string()) {
                let _ = grandchild.kill();
                let _ = grandchild.wait();
                return Err(error);
            }
            remain_alive();
            let _ = grandchild.kill();
            let _ = grandchild.wait();
            // Kept alive until this explicit point; it is never inherited.
            drop(nested);
            Ok(())
        }
        "sibling" => {
            remain_alive();
            Ok(())
        }
        "grandchild" => {
            publish(root, "grandchild.ready", "ready")?;
            remain_alive();
            Ok(())
        }
        "idle" => {
            publish(root, "idle.pid", std::process::id().to_string())?;
            remain_alive();
            Ok(())
        }
        "finish" => {
            publish(root, "finish.ready", "ready")?;
            let deadline = Instant::now() + FIXTURE_LIFETIME;
            while !root.join("finish.allowed").exists() {
                if Instant::now() >= deadline {
                    return Err(io::Error::new(io::ErrorKind::TimedOut, "fixture exit was not released"));
                }
                thread::sleep(Duration::from_millis(10));
            }
            publish(root, "finish.completed", "completed")
        }
        _ => Err(io::Error::new(io::ErrorKind::InvalidInput, "unknown fixture role")),
    }
}
