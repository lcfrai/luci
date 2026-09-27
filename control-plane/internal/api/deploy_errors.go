package api

// The one place a deploy.Error becomes a status code. It is one place on
// purpose: the mapping is a contract the desktop switches on, and a second copy
// in a handler is a second answer to "what does a rejected key look like".

import (
	"errors"
	"net/http"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

// upstreamPanacea names the remote in details.upstream. The desktop reads it to
// say WHICH system is down, because a 503 from this surface can mean Panacea or
// the Hub and the two have different answers.
const upstreamPanacea = "panacea"

// deployNotConnectedMessage is the one sentence a route answers when there is no
// Panacea to call: no address, no key, or no Manager wired at all.
const deployNotConnectedMessage = "Connect to Panacea first."

// The four sentences a KindUnavailable becomes. They differ on the one thing a
// user needs and the status code cannot say: whether the request may already
// have LANDED. A timeout may have; a refusal before the act cannot have.
const (
	unavailableSilent  = "Panacea did not answer, so the cockpit cannot tell whether it acted."
	unavailableGarbled = "Panacea answered, but not in a form the cockpit can use, so it cannot tell what Panacea did."
	unavailableErrored = "Panacea answered with an error. Look at the history before trying again."
	unavailableRefused = "Panacea, or a proxy in front of it, refused the request before acting, so nothing was changed."
)

// unavailableSentence chooses by the status class, the same five task 13's
// upstreamAnswer names. 502 and 504 join 0: a gateway that gave up says nothing
// about whether what it was in front of acted.
func unavailableSentence(status int) string {
	switch {
	case status == 0, status == http.StatusBadGateway, status == http.StatusGatewayTimeout:
		return unavailableSilent
	case status >= 200 && status <= 299:
		return unavailableGarbled
	case status >= 500:
		return unavailableErrored
	default:
		return unavailableRefused
	}
}

// deployFallback is the sentence used when the target sent none. An empty
// message renders as a blank error in the cockpit, which tells a user nothing
// and looks like a bug in the cockpit rather than a refusal by Panacea.
var deployFallback = map[deploy.Kind]string{
	deploy.KindUnauthorized: "Panacea refused this key.",
	deploy.KindForbidden:    "Panacea refused this act for your account.",
	deploy.KindNotFound:     "Panacea has no such project, run or deployment.",
	deploy.KindConflict:     "Panacea refused the request because of its current state.",
	deploy.KindRejected:     "Panacea refused the request.",
}

// deployProblem is the table itself: a Kind, and the status and code it answers
// with. ok is false for a Kind no row covers, which is a 500 and not a guess.
func deployProblem(e *deploy.Error) (status int, code, message string, ok bool) {
	switch e.Kind {
	case deploy.KindNotConnected:
		return http.StatusConflict, httpx.CodeDeployNotConnected, deployNotConnectedMessage, true
	case deploy.KindUnavailable:
		return http.StatusServiceUnavailable, httpx.CodeUpstreamUnavailable, unavailableSentence(e.Status), true
	case deploy.KindUnauthorized:
		// 409, not 401: a 401 on this surface already means the LOCAL caller key
		// is wrong, or that the Hub session is out. A third meaning would make
		// the status useless to the desktop.
		return http.StatusConflict, httpx.CodeDeployTokenRejected, e.Message, true
	case deploy.KindForbidden:
		return http.StatusForbidden, httpx.CodeDeployForbidden, e.Message, true
	case deploy.KindNotFound:
		return http.StatusNotFound, httpx.CodeDeployNotFound, e.Message, true
	case deploy.KindConflict:
		return http.StatusConflict, httpx.CodeDeployConflict, e.Message, true
	case deploy.KindRejected:
		return http.StatusUnprocessableEntity, httpx.CodeDeployRejected, e.Message, true
	}
	return 0, "", "", false
}

// writeDeployError answers err as a problem.
//
// It matches on *deploy.Error rather than on deploy.KindOf, and the difference
// is load-bearing: KindOf answers KindUnavailable for ANY other non-nil error,
// which is the right default for a caller deciding whether to retry and the
// wrong one here. A nil map inside this process is not Panacea being down, and
// dressing it as a 503 sends the user away to wait instead of leaving a 500 in
// the log where the bug is.
//
// Every problem it writes carries details.upstream and details.status, the
// latter a JSON NUMBER: the desktop reads status 0 as "Panacea may have acted".
// extra is merged over the top and can add to those two but never replace them,
// so a caller passing {"reserved":true} tells the user what was reserved
// without being able to rewrite what Panacea answered.
func (a *API) writeDeployError(w http.ResponseWriter, r *http.Request, op string, err error, extra ...map[string]any) {
	var e *deploy.Error
	if !errors.As(err, &e) || e == nil {
		a.internal(w, r, op, err)
		return
	}
	status, code, message, ok := deployProblem(e)
	if !ok {
		a.internal(w, r, op, err)
		return
	}
	if message == "" {
		message = deployFallback[e.Kind]
	}

	details := map[string]any{"upstream": upstreamPanacea, "status": e.Status}
	if e.RetryAfter > 0 {
		details["retryAfter"] = int(e.RetryAfter.Round(time.Second) / time.Second)
	}
	for _, m := range extra {
		for k, v := range m {
			if k == "upstream" || k == "status" {
				continue
			}
			details[k] = v
		}
	}

	// Only the unreachable case is logged, and at warn. The others are the
	// user's own answer, shown to them in the cockpit; a log line per 404 buries
	// the one line an operator is looking for. e.Error() carries the op, the
	// status and Panacea's sentence - never the key, which no deploy.Error holds.
	if e.Kind == deploy.KindUnavailable {
		a.deps.Log.Warn("api: "+op+" could not reach Panacea",
			"method", r.Method, "path", r.URL.Path, "status", e.Status, "err", e)
	}
	httpx.WriteProblemDetails(w, status, code, message, details)
}
