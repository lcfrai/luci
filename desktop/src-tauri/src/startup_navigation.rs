//! One initial-document recovery. Never reload a frontend that has bootstrapped.
use std::sync::{Arc, Condvar, Mutex};
use std::time::Duration;
use tauri::{Manager, Url, WebviewUrl};

const FIRST_CHECK: Duration = Duration::from_secs(15);
const FINAL_CHECK: Duration = Duration::from_secs(45);
const FAILURE_TITLE: &str = "Luci could not load — close and reopen Luci";
const FAILURE_MESSAGE: &str = "Luci could not load its startup page. Close and reopen Luci.";
const REPLACED_MESSAGE: &str =
    "This startup page was replaced. Wait for Luci's new window content.";

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum Phase {
    Waiting,
    Recovering,
    Observed,
    Failed,
    Stopped,
}

#[derive(Debug)]
struct Gate {
    phase: Phase,
    retried: bool,
}

#[derive(Debug, PartialEq, Eq)]
enum Action {
    None,
    Navigate,
    Fail,
}

impl Gate {
    fn require_current_document(&self, is_replacement: bool) -> Result<(), &'static str> {
        if self.retried
            && matches!(self.phase, Phase::Waiting | Phase::Recovering)
            && !is_replacement
        {
            Err(REPLACED_MESSAGE)
        } else {
            Ok(())
        }
    }

    fn initial_check(&mut self, url: Option<&str>, is_app: bool) -> Action {
        if self.phase != Phase::Waiting {
            return Action::None;
        }
        if url == Some("about:blank") && !self.retried {
            self.retried = true;
            self.phase = Phase::Recovering;
            Action::Navigate
        } else if is_app {
            // A loaded app document is never automatically refreshed.
            Action::None
        } else {
            self.phase = Phase::Failed;
            Action::Fail
        }
    }

    fn admit(&mut self, is_app: bool) -> Result<(), &'static str> {
        match self.phase {
            Phase::Observed => Ok(()),
            Phase::Waiting if is_app => {
                self.phase = Phase::Observed;
                Ok(())
            }
            Phase::Recovering => {
                Err("Luci is reloading its startup page. Wait a moment, then choose Retry.")
            }
            Phase::Stopped => Err("Luci is closing."),
            _ => {
                self.phase = Phase::Failed;
                Err(FAILURE_MESSAGE)
            }
        }
    }

    fn page_finished(&mut self, is_app: bool) {
        if self.phase == Phase::Recovering && is_app {
            self.phase = Phase::Waiting;
        }
    }

    fn expire(&mut self) -> Action {
        if matches!(self.phase, Phase::Waiting | Phase::Recovering) {
            self.phase = Phase::Failed;
            Action::Fail
        } else {
            Action::None
        }
    }

    fn done(&self) -> bool {
        matches!(self.phase, Phase::Observed | Phase::Failed | Phase::Stopped)
    }
}

#[derive(Clone)]
pub(crate) struct StartupNavigation {
    target: Option<Url>,
    recovery_target: Option<Url>,
    shared: Arc<(Mutex<Gate>, Condvar)>,
}

impl StartupNavigation {
    pub(crate) fn new(config: &tauri::Config, development: bool) -> Self {
        // Only the configured packaged main entrypoint is eligible. Development
        // and explicitly external/custom window URLs retain their own behavior.
        let packaged = !development
            && !matches!(
                &config.build.frontend_dist,
                Some(tauri::utils::config::FrontendDist::Url(_))
            );
        let target = config.app.windows.iter().find(|window| window.label == "main")
            .filter(|window| packaged && matches!(&window.url, WebviewUrl::App(path) if path == std::path::Path::new("index.html")))
            .and_then(|window| {
                let origin = if cfg!(windows) {
                    if window.use_https_scheme { "https://tauri.localhost/" } else { "http://tauri.localhost/" }
                } else {
                    "tauri://localhost/"
                };
                origin.parse().ok()
            });
        let recovery_target = target.as_ref().map(|target: &Url| {
            let mut url = target.clone();
            // A late Finished event for the original document must not admit
            // that old document while its replacement navigation is pending.
            url.set_query(Some(&format!(
                "luci-startup-recovery={}",
                uuid::Uuid::new_v4()
            )));
            url
        });
        Self {
            target,
            recovery_target,
            shared: Arc::new((
                Mutex::new(Gate {
                    phase: Phase::Waiting,
                    retried: false,
                }),
                Condvar::new(),
            )),
        }
    }

    fn is_app(&self, url: &Url) -> bool {
        self.target.as_ref().is_some_and(|target| {
            url.scheme() == target.scheme()
                && url.host_str() == target.host_str()
                && url.port() == target.port()
                && url.username().is_empty()
                && url.password().is_none()
                && matches!(url.path(), "/" | "/index.html")
        })
    }

    pub(crate) async fn admit_frontend(&self, window: &tauri::WebviewWindow) -> Result<(), String> {
        if window.label() != "main" {
            return Err("Startup metadata is available only to Luci's main window.".into());
        }
        if self.target.is_none() {
            return Ok(());
        }
        // Capture the observed document once. A stale original/blank document
        // cannot become the replacement merely because Finished arrives later.
        let document = window.url().ok();
        let is_app = document.as_ref().is_some_and(|url| self.is_app(url));
        let is_replacement = document
            .as_ref()
            .is_some_and(|url| self.recovery_target.as_ref() == Some(url));
        {
            let (lock, changed) = &*self.shared;
            let mut gate = lock.lock().map_err(|_| FAILURE_MESSAGE)?;
            // Reject without changing phase, including when Finished raced this
            // lock and has already returned the replacement to Waiting.
            gate.require_current_document(is_replacement)
                .map_err(str::to_owned)?;
            if gate.phase != Phase::Recovering {
                // Latch healthy bootstrap synchronously, before scheduling any
                // worker; the timer cannot overtake an already admitted page.
                let result = gate.admit(is_app);
                let failed = gate.phase == Phase::Failed;
                drop(gate);
                changed.notify_all();
                if failed {
                    let _ = window.set_title(FAILURE_TITLE);
                }
                return result.map_err(str::to_owned);
            }
        }
        let navigation = self.clone();
        // A module script may invoke runtime_info before PageLoadFinished.
        // Wait off the UI thread, so a replacement document can finish loading
        // without a spurious Retry screen or admitting its old predecessor.
        let document = document.ok_or(REPLACED_MESSAGE)?;
        let result = tauri::async_runtime::spawn_blocking(move || {
            navigation.finish_frontend_admission(document)
        })
        .await
        .map_err(|_| FAILURE_MESSAGE.to_owned())?;
        if result.is_err() {
            let _ = window.set_title(FAILURE_TITLE);
        }
        result
    }

    fn finish_frontend_admission(&self, document: Url) -> Result<(), String> {
        if self.recovery_target.as_ref() != Some(&document) {
            // Never wait and then poison/admit the replacement using a stale
            // boolean captured from a different document.
            return Err(REPLACED_MESSAGE.into());
        }
        let (lock, changed) = &*self.shared;
        let gate = lock.lock().map_err(|_| FAILURE_MESSAGE)?;
        let (mut gate, _) = changed
            .wait_timeout_while(gate, FINAL_CHECK, |gate| gate.phase == Phase::Recovering)
            .map_err(|_| FAILURE_MESSAGE)?;
        if gate.phase == Phase::Recovering {
            gate.expire();
        }
        gate.require_current_document(true).map_err(str::to_owned)?;
        let result = gate.admit(true);
        changed.notify_all();
        result.map_err(str::to_owned)
    }

    pub(crate) fn page_finished(&self, label: &str, url: &Url) {
        if label != "main" {
            return;
        }
        let (lock, changed) = &*self.shared;
        if let Ok(mut gate) = lock.lock() {
            gate.page_finished(self.recovery_target.as_ref() == Some(url));
            changed.notify_all();
        }
    }

    pub(crate) fn stop(&self) {
        let (lock, changed) = &*self.shared;
        if let Ok(mut gate) = lock.lock() {
            gate.phase = Phase::Stopped;
            changed.notify_all();
        }
    }

    fn wait(&self, duration: Duration) -> bool {
        let (lock, changed) = &*self.shared;
        let Ok(gate) = lock.lock() else {
            return false;
        };
        let Ok((gate, _)) = changed.wait_timeout_while(gate, duration, |gate| !gate.done()) else {
            return false;
        };
        !gate.done()
    }

    fn check(&self, app: &tauri::AppHandle, terminal: bool) {
        let Some(window) = app.get_webview_window("main") else {
            self.stop();
            return;
        };
        let url = window.url().ok();
        let (lock, changed) = &*self.shared;
        let Ok(mut gate) = lock.lock() else {
            let _ = window.set_title(FAILURE_TITLE);
            return;
        };
        let action = if terminal {
            gate.expire()
        } else {
            gate.initial_check(
                url.as_ref().map(Url::as_str),
                url.as_ref().is_some_and(|url| self.is_app(url)),
            )
        };
        // Bootstrap admission and the recovery request share this lock. An old
        // document cannot enter App while a replacement navigation is committed.
        if action == Action::Navigate {
            if self
                .recovery_target
                .clone()
                .is_none_or(|target| window.navigate(target).is_err())
            {
                gate.phase = Phase::Failed;
                let _ = window.set_title(FAILURE_TITLE);
            }
        } else if action == Action::Fail {
            // This remains visible even when the WebView cannot render HTML.
            // Never inject a fallback document into an unknown origin.
            let _ = window.set_title(FAILURE_TITLE);
        }
        changed.notify_all();
    }

    pub(crate) fn install(&self, app: &tauri::AppHandle) {
        if self.target.is_none() {
            return;
        }
        let navigation = self.clone();
        let handle = app.clone();
        if std::thread::Builder::new()
            .name("luci-initial-navigation".into())
            .spawn(move || {
                let started = std::time::Instant::now();
                if !navigation.wait(FIRST_CHECK) {
                    return;
                }
                let first = navigation.clone();
                let first_handle = handle.clone();
                if handle
                    .run_on_main_thread(move || {
                        first.check(&first_handle, started.elapsed() >= FINAL_CHECK)
                    })
                    .is_err()
                {
                    navigation.stop();
                    return;
                }
                if !navigation.wait(FINAL_CHECK.saturating_sub(started.elapsed())) {
                    return;
                }
                let last = navigation.clone();
                let last_handle = handle.clone();
                let _ = handle.run_on_main_thread(move || last.check(&last_handle, true));
            })
            .is_err()
        {
            self.check(app, true);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn gate() -> Gate {
        Gate {
            phase: Phase::Waiting,
            retried: false,
        }
    }

    #[test]
    fn observed_bootstrap_permanently_prevents_reload_and_timeout() {
        let mut gate = gate();
        assert_eq!(gate.admit(true), Ok(()));
        assert_eq!(gate.initial_check(Some("about:blank"), false), Action::None);
        assert_eq!(gate.expire(), Action::None);
        assert!(!gate.retried);
    }

    #[test]
    fn recovery_committed_before_bootstrap_refuses_old_document() {
        let mut gate = gate();
        assert_eq!(
            gate.initial_check(Some("about:blank"), false),
            Action::Navigate
        );
        assert!(gate.admit(true).is_err());
        assert_eq!(gate.phase, Phase::Recovering);
        gate.page_finished(false);
        assert!(gate.admit(true).is_err());
        gate.page_finished(true);
        assert_eq!(gate.admit(true), Ok(()));
        assert_eq!(gate.expire(), Action::None);
    }

    #[test]
    fn blank_after_one_recovery_fails_without_another_navigation() {
        let mut gate = gate();
        assert_eq!(
            gate.initial_check(Some("about:blank"), false),
            Action::Navigate
        );
        gate.page_finished(true);
        assert_eq!(gate.initial_check(Some("about:blank"), false), Action::Fail);
        assert!(gate.admit(true).is_err());
    }

    #[test]
    fn unknown_or_other_url_fails_without_reloading() {
        for url in [
            None,
            Some("https://example.com/"),
            Some("about:blank#other"),
        ] {
            let mut gate = gate();
            assert_eq!(gate.initial_check(url, false), Action::Fail);
            assert!(!gate.retried);
            assert!(gate.admit(true).is_err());
        }
    }

    #[test]
    fn existing_app_document_is_not_refreshed_while_starting() {
        let mut gate = gate();
        assert_eq!(
            gate.initial_check(Some("http://tauri.localhost/"), true),
            Action::None
        );
        assert!(!gate.retried);
        assert_eq!(gate.expire(), Action::Fail);
    }

    #[test]
    fn closing_cancels_recovery_and_bootstrap() {
        let mut gate = gate();
        gate.phase = Phase::Stopped;
        assert!(gate.done());
        assert_eq!(gate.initial_check(Some("about:blank"), false), Action::None);
        assert_eq!(gate.expire(), Action::None);
        assert!(gate.admit(true).is_err());
    }

    #[test]
    fn app_origin_check_preserves_local_parameters_but_rejects_other_documents() {
        let navigation = StartupNavigation {
            target: Some("http://tauri.localhost/".parse().unwrap()),
            recovery_target: Some(
                "http://tauri.localhost/?luci-startup-recovery=expected"
                    .parse()
                    .unwrap(),
            ),
            shared: Arc::new((Mutex::new(gate()), Condvar::new())),
        };
        assert!(navigation.is_app(&"http://tauri.localhost/".parse().unwrap()));
        assert!(navigation.is_app(&"http://tauri.localhost/index.html".parse().unwrap()));
        assert!(navigation.is_app(navigation.recovery_target.as_ref().unwrap()));
        assert!(navigation.is_app(
            &"http://tauri.localhost/?view=local#local-route"
                .parse()
                .unwrap()
        ));
        for url in [
            "https://tauri.localhost/",
            "http://tauri.localhost:99/",
            "http://other/",
            "http://tauri.localhost/other.html",
        ] {
            assert!(!navigation.is_app(&url.parse().unwrap()));
        }
    }

    #[test]
    fn only_the_replacement_document_can_release_committed_recovery() {
        let navigation = StartupNavigation {
            target: Some("http://tauri.localhost/".parse().unwrap()),
            recovery_target: Some(
                "http://tauri.localhost/?luci-startup-recovery=expected"
                    .parse()
                    .unwrap(),
            ),
            shared: Arc::new((Mutex::new(gate()), Condvar::new())),
        };
        navigation
            .shared
            .0
            .lock()
            .unwrap()
            .initial_check(Some("about:blank"), false);
        navigation.page_finished("main", navigation.target.as_ref().unwrap());
        assert_eq!(navigation.shared.0.lock().unwrap().phase, Phase::Recovering);
        navigation.page_finished("other", navigation.recovery_target.as_ref().unwrap());
        assert_eq!(navigation.shared.0.lock().unwrap().phase, Phase::Recovering);
        navigation.page_finished("main", navigation.recovery_target.as_ref().unwrap());
        assert_eq!(navigation.shared.0.lock().unwrap().admit(true), Ok(()));
        navigation.page_finished("main", navigation.target.as_ref().unwrap());
        assert_eq!(navigation.shared.0.lock().unwrap().expire(), Action::None);
    }

    #[test]
    fn stop_wakes_the_finite_startup_waiter_without_waiting_for_timeout() {
        let navigation = StartupNavigation {
            target: None,
            recovery_target: None,
            shared: Arc::new((Mutex::new(gate()), Condvar::new())),
        };
        navigation.stop();
        assert!(!navigation.wait(Duration::from_secs(45)));
    }

    #[test]
    fn development_and_external_main_windows_do_not_install_recovery() {
        let mut config = tauri::Config::default();
        let mut window = tauri::utils::config::WindowConfig::default();
        window.label = "main".into();
        config.app.windows = vec![window];
        let packaged = StartupNavigation::new(&config, false);
        assert!(packaged.target.is_some());
        assert_ne!(packaged.target, packaged.recovery_target);
        assert!(StartupNavigation::new(&config, true).target.is_none());
        config.app.windows[0].url = WebviewUrl::External("https://example.com/".parse().unwrap());
        assert!(StartupNavigation::new(&config, false).target.is_none());
        config.app.windows[0].url = WebviewUrl::App("other.html".into());
        assert!(StartupNavigation::new(&config, false).target.is_none());
        config.app.windows[0].url = WebviewUrl::App("index.html".into());
        config.build.frontend_dist = Some(tauri::utils::config::FrontendDist::Url(
            "https://example.com/".parse().unwrap(),
        ));
        assert!(StartupNavigation::new(&config, false).target.is_none());
    }

    #[test]
    fn replacement_bootstrap_waits_for_its_finished_event_then_disarms_timer() {
        let navigation = StartupNavigation {
            target: Some("http://tauri.localhost/".parse().unwrap()),
            recovery_target: Some(
                "http://tauri.localhost/?luci-startup-recovery=expected"
                    .parse()
                    .unwrap(),
            ),
            shared: Arc::new((Mutex::new(gate()), Condvar::new())),
        };
        navigation
            .shared
            .0
            .lock()
            .unwrap()
            .initial_check(Some("about:blank"), false);
        let pending = navigation.clone();
        let (sent, received) = std::sync::mpsc::channel();
        let document = navigation.recovery_target.clone().unwrap();
        let worker = std::thread::spawn(move || {
            sent.send(pending.finish_frontend_admission(document))
                .unwrap();
        });
        let before_finish = received.try_recv();
        navigation.page_finished("main", navigation.recovery_target.as_ref().unwrap());
        let after_finish = received.recv_timeout(Duration::from_secs(2));
        if after_finish.is_err() {
            navigation.stop();
        }
        worker.join().unwrap();
        assert!(before_finish.is_err());
        assert_eq!(after_finish.unwrap(), Ok(()));
        assert_eq!(navigation.shared.0.lock().unwrap().expire(), Action::None);
    }

    #[test]
    fn stale_blank_and_original_bootstraps_cannot_poison_or_admit_replacement() {
        let navigation = StartupNavigation {
            target: Some("http://tauri.localhost/".parse().unwrap()),
            recovery_target: Some(
                "http://tauri.localhost/?luci-startup-recovery=expected"
                    .parse()
                    .unwrap(),
            ),
            shared: Arc::new((Mutex::new(gate()), Condvar::new())),
        };
        navigation
            .shared
            .0
            .lock()
            .unwrap()
            .initial_check(Some("about:blank"), false);
        for document in ["about:blank", "http://tauri.localhost/"] {
            let document: Url = document.parse().unwrap();
            assert!(navigation
                .finish_frontend_admission(document.clone())
                .is_err());
            let gate = navigation.shared.0.lock().unwrap();
            assert_eq!(gate.phase, Phase::Recovering);
            assert!(gate.require_current_document(false).is_err());
        }
        navigation.page_finished("main", navigation.recovery_target.as_ref().unwrap());
        for document in ["about:blank", "http://tauri.localhost/"] {
            assert!(navigation
                .finish_frontend_admission(document.parse().unwrap())
                .is_err());
            let gate = navigation.shared.0.lock().unwrap();
            assert_eq!(gate.phase, Phase::Waiting);
            assert!(gate.require_current_document(false).is_err());
        }
        assert_eq!(
            navigation.finish_frontend_admission(navigation.recovery_target.clone().unwrap()),
            Ok(())
        );
        assert_eq!(navigation.shared.0.lock().unwrap().phase, Phase::Observed);
        assert_eq!(navigation.shared.0.lock().unwrap().expire(), Action::None);
    }

    #[test]
    fn observed_replacement_allows_normal_refresh_without_rearming_recovery() {
        let mut gate = gate();
        assert_eq!(
            gate.initial_check(Some("about:blank"), false),
            Action::Navigate
        );
        gate.page_finished(true);
        assert_eq!(gate.require_current_document(true), Ok(()));
        assert_eq!(gate.admit(true), Ok(()));
        // The successfully bootstrapped app may subsequently refresh or remove
        // the ephemeral query. That is no longer initial recovery admission.
        assert_eq!(gate.require_current_document(false), Ok(()));
        assert_eq!(gate.admit(true), Ok(()));
        assert_eq!(gate.initial_check(Some("about:blank"), false), Action::None);
        assert_eq!(gate.expire(), Action::None);
        assert_eq!(gate.phase, Phase::Observed);
        assert!(gate.retried);
        // Terminal phases retain their own errors rather than a stale-page error.
        for phase in [Phase::Failed, Phase::Stopped] {
            gate.phase = phase;
            assert_eq!(gate.require_current_document(false), Ok(()));
            assert!(gate.admit(true).is_err());
            assert_eq!(gate.phase, phase);
        }
    }
}
