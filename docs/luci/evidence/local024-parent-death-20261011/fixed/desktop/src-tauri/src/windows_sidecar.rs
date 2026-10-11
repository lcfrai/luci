//! Own only the managed Windows sidecar tree, including abrupt desktop termination.
//! JOB_LIST assigns the process to our job inside CreateProcessW. Spawn-then-assign
//! (even CREATE_SUSPENDED) leaves an orphan window if the desktop dies in between.
use std::{
    cmp::Ordering as CmpOrdering,
    ffi::OsStr,
    fs::OpenOptions,
    io,
    mem::size_of,
    os::windows::{ffi::OsStrExt, io::{AsRawHandle, FromRawHandle, OwnedHandle}},
    process::Command,
    ptr,
    sync::{atomic::{AtomicBool, Ordering}, Arc},
};
use windows_sys::Win32::{
    Foundation::{SetHandleInformation, HANDLE_FLAG_INHERIT, WAIT_OBJECT_0},
    Globalization::{CompareStringOrdinal, CSTR_EQUAL, CSTR_GREATER_THAN, CSTR_LESS_THAN},
    System::{
        JobObjects::{CreateJobObjectW, SetInformationJobObject, TerminateJobObject,
            JobObjectExtendedLimitInformation, JOBOBJECT_EXTENDED_LIMIT_INFORMATION,
            JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
        Threading::{CreateProcessW, DeleteProcThreadAttributeList, InitializeProcThreadAttributeList,
            UpdateProcThreadAttribute, WaitForSingleObject, CREATE_NO_WINDOW,
            CREATE_UNICODE_ENVIRONMENT, EXTENDED_STARTUPINFO_PRESENT, INFINITE,
            LPPROC_THREAD_ATTRIBUTE_LIST, PROCESS_INFORMATION, PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
            PROC_THREAD_ATTRIBUTE_JOB_LIST, STARTF_USESTDHANDLES, STARTUPINFOEXW},
    },
};
#[path = "windows_command_line.rs"]
mod windows_command_line;

pub(crate) struct OwnedChild {
    // No waiter or child receives a copy of this handle. Native process death
    // closes its sole owner even when Tauri's normal Exit callback never runs.
    job: OwnedHandle,
    #[cfg(test)]
    pid: u32,
}
impl OwnedChild {
    pub(crate) fn kill(self) -> io::Result<()> {
        if unsafe { TerminateJobObject(self.job.as_raw_handle(), 1) } == 0 {
            return Err(io::Error::last_os_error());
        }
        Ok(())
    }
    #[cfg(test)]
    pub(crate) fn pid(&self) -> u32 { self.pid }
}

fn new_job() -> io::Result<OwnedHandle> {
    let raw = unsafe { CreateJobObjectW(ptr::null(), ptr::null()) };
    if raw.is_null() { return Err(io::Error::last_os_error()); }
    let job = unsafe { OwnedHandle::from_raw_handle(raw) };
    if unsafe { SetHandleInformation(raw, HANDLE_FLAG_INHERIT, 0) } == 0 {
        return Err(io::Error::last_os_error());
    }
    let mut limits = JOBOBJECT_EXTENDED_LIMIT_INFORMATION::default();
    limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
    if unsafe { SetInformationJobObject(raw, JobObjectExtendedLimitInformation,
        (&limits as *const JOBOBJECT_EXTENDED_LIMIT_INFORMATION).cast(), size_of::<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>() as u32) } == 0 {
        return Err(io::Error::last_os_error());
    }
    Ok(job)
}

struct Attributes { storage: Vec<usize> }
impl Attributes {
    fn new() -> io::Result<Self> {
        let mut bytes = 0;
        unsafe { InitializeProcThreadAttributeList(ptr::null_mut(), 2, 0, &mut bytes); }
        if bytes == 0 { return Err(io::Error::last_os_error()); }
        let mut storage = vec![0usize; bytes.div_ceil(size_of::<usize>())];
        if unsafe { InitializeProcThreadAttributeList(storage.as_mut_ptr().cast(), 2, 0, &mut bytes) } == 0 {
            return Err(io::Error::last_os_error());
        }
        Ok(Self { storage })
    }
    fn as_ptr(&mut self) -> LPPROC_THREAD_ATTRIBUTE_LIST { self.storage.as_mut_ptr().cast() }
    // The referenced arrays remain in spawn_inner until CreateProcessW returns.
    fn handles(&mut self, attribute: u32, handles: &[std::os::windows::io::RawHandle]) -> io::Result<()> {
        if unsafe { UpdateProcThreadAttribute(self.as_ptr(), 0, attribute as usize,
            handles.as_ptr().cast(), std::mem::size_of_val(handles), ptr::null_mut(), ptr::null()) } == 0 {
            return Err(io::Error::last_os_error());
        }
        Ok(())
    }
}
impl Drop for Attributes {
    fn drop(&mut self) { unsafe { DeleteProcThreadAttributeList(self.as_ptr()); } }
}

fn wide(value: &OsStr) -> io::Result<Vec<u16>> {
    let result: Vec<_> = value.encode_wide().collect();
    if result.contains(&0) { return Err(io::Error::new(io::ErrorKind::InvalidInput, "NUL in sidecar configuration")); }
    Ok(result)
}
fn key_cmp(a: &[u16], b: &[u16]) -> CmpOrdering {
    // Valid Windows environment names fit i32; comparisons ignore case exactly
    // as Windows environment lookup does, without lossy Unicode conversion.
    match unsafe { CompareStringOrdinal(a.as_ptr(), a.len() as i32, b.as_ptr(), b.len() as i32, 1) } {
        CSTR_LESS_THAN => CmpOrdering::Less,
        CSTR_EQUAL => CmpOrdering::Equal,
        CSTR_GREATER_THAN => CmpOrdering::Greater,
        _ => unreachable!("Valid environment key comparison failed"),
    }
}
fn environment(command: &Command) -> io::Result<Vec<u16>> {
    // The managed caller inherits the environment and sets only explicit env
    // overrides. It never uses env_clear; token values stay in this buffer only.
    let mut rows = std::env::vars_os().map(|(k,v)| Ok((wide(&k)?,wide(&v)?))).collect::<io::Result<Vec<_>>>()?;
    for (key, value) in command.get_envs() {
        let key = wide(key)?;
        if key.is_empty() || key.contains(&(b'=' as u16)) {
            return Err(io::Error::new(io::ErrorKind::InvalidInput, "Invalid sidecar environment name"));
        }
        rows.retain(|(existing,_)| key_cmp(existing,&key) != CmpOrdering::Equal);
        if let Some(value) = value { rows.push((key,wide(value)?)); }
    }
    rows.sort_by(|(a,_),(b,_)| key_cmp(a,b));
    let mut block = Vec::new();
    for (key, value) in rows { block.extend(key); block.push(b'=' as u16); block.extend(value); block.push(0); }
    if block.is_empty() { block.push(0); }
    block.push(0);
    Ok(block)
}

pub(crate) fn spawn(command: Command, exited: Arc<AtomicBool>) -> io::Result<OwnedChild> {
    spawn_inner(command, exited, |_| Ok(()))
}

fn spawn_inner(command: Command, exited: Arc<AtomicBool>, after_create: impl FnOnce(u32) -> io::Result<()>) -> io::Result<OwnedChild> {
    // Tauri still resolves the exact bundled sidecar executable. No shell or PATH
    // lookup is introduced. Every unsupported capability fails startup closed.
    let program = wide(command.get_program())?;
    let args = command.get_args().map(wide).collect::<io::Result<Vec<_>>>()?;
    let mut line = windows_command_line::command_line(&program, &args)?;
    let mut executable = program; executable.push(0);
    let mut env = environment(&command)?;
    let directory = command.get_current_dir().map(|p| wide(p.as_os_str())).transpose()?.map(|mut p| { p.push(0); p });
    let job = new_job()?;
    // The old event loop discarded both output streams. NUL provides the same
    // bounded disposal without pipes that can fill or waiter threads retaining
    // ownership. Only this harmless handle is inherited, never the job handle.
    let null = OpenOptions::new().read(true).write(true).open("NUL")?;
    if unsafe { SetHandleInformation(null.as_raw_handle(), HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT) } == 0 {
        return Err(io::Error::last_os_error());
    }
    let jobs = [job.as_raw_handle()];
    let stdio = [null.as_raw_handle()];
    let mut attributes = Attributes::new()?;
    attributes.handles(PROC_THREAD_ATTRIBUTE_JOB_LIST, &jobs)?;
    attributes.handles(PROC_THREAD_ATTRIBUTE_HANDLE_LIST, &stdio)?;
    let mut startup = STARTUPINFOEXW::default();
    startup.StartupInfo.cb = size_of::<STARTUPINFOEXW>() as u32;
    startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
    startup.StartupInfo.hStdInput = null.as_raw_handle();
    startup.StartupInfo.hStdOutput = null.as_raw_handle();
    startup.StartupInfo.hStdError = null.as_raw_handle();
    startup.lpAttributeList = attributes.as_ptr();
    let mut process = PROCESS_INFORMATION::default();
    if unsafe { CreateProcessW(executable.as_ptr(), line.as_mut_ptr(), ptr::null(), ptr::null(), 1,
        EXTENDED_STARTUPINFO_PRESENT | CREATE_UNICODE_ENVIRONMENT | CREATE_NO_WINDOW,
        env.as_mut_ptr().cast(), directory.as_ref().map_or(ptr::null(), |v| v.as_ptr()),
        &startup.StartupInfo, &mut process) } == 0 {
        return Err(io::Error::last_os_error());
    }
    let process_handle = unsafe { OwnedHandle::from_raw_handle(process.hProcess) };
    let thread_handle = unsafe { OwnedHandle::from_raw_handle(process.hThread) };
    drop(thread_handle);
    // Test-only callers can stop the disposable parent at this exact boundary.
    // Job ownership already exists inside the kernel, before any Rust followup.
    after_create(process.dwProcessId)?;
    std::thread::Builder::new().name("luci-sidecar-exit".into()).spawn(move || {
        if unsafe { WaitForSingleObject(process_handle.as_raw_handle(), INFINITE) } == WAIT_OBJECT_0 {
            exited.store(true, Ordering::Release);
        }
    })?;
    Ok(OwnedChild { job, #[cfg(test)] pid: process.dwProcessId })
}

#[cfg(test)]
#[path = "windows_sidecar_tests.rs"]
mod tests;
