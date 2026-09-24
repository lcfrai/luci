package harness

import (
	"fmt"
	"regexp"
)

// claudePermissionFlag maps the product's three-word vocabulary onto the CLI's
// --permission-mode choices. Evidence: `claude --help` 2.1.267 lists
// acceptEdits, auto, bypassPermissions, manual, dontAsk and plan.
//
// "ask" is deliberately absent. The choice list has no `default`, and --help
// does not say which of manual/auto/dontAsk is the out-of-the-box behaviour.
// Emitting no flag gets the CLI's own default, which IS the ask behaviour, and
// is provably correct without guessing a mapping. Contract open question 1
// tracks confirming it against a real session before Phase 8 adds an admin
// control over permitted modes.
var claudePermissionFlag = map[PermissionMode]string{
	PermissionBypass: "bypassPermissions",
	PermissionPlan:   "plan",
}

// Build turns a Request into the command and argv a sandbox runs.
//
// The opening prompt is appended as ONE argv element and is never split,
// joined, quoted or escaped. That is the whole point of this function: the
// legacy comma-joined PANACEA_APP_ARGS turned "add tests, then refactor" into
// two arguments, and Group A task 3 replaced the transport with
// PANACEA_APP_ARGS_JSON so the producer could stay honest.
//
// Validation order is fixed — unknown harness, unavailable, then model,
// effort, permission, resume and conversation, prompt, in that order — so a
// request with two faults fails the same way every run. That is not Request's
// declaration order: the struct declares OpeningPrompt before Resume, but
// Build checks resume before prompt, because this list is the contract's
// stated order, not an accident of the struct layout. On any error both return
// values are zero: a partial argv is how a rejected request becomes a started
// session.
func Build(r Request) (command string, argv []string, err error) {
	d, ok := Lookup(r.Harness)
	if !ok {
		return "", nil, fmt.Errorf("%w: %q", ErrUnknownHarness, r.Harness)
	}
	if !d.Available() {
		return "", nil, fmt.Errorf("%w: %q: %s", ErrUnavailable, d.ID, d.Note)
	}
	if err := validateModel(d, r.Model); err != nil {
		return "", nil, err
	}
	if err := validateEffort(d, r.Effort); err != nil {
		return "", nil, err
	}
	if err := validatePermission(d, r.Permission); err != nil {
		return "", nil, err
	}
	if err := validateResume(d, r.Resume, r.Conversation); err != nil {
		return "", nil, err
	}
	if err := validatePrompt(d, r.OpeningPrompt); err != nil {
		return "", nil, err
	}

	switch d.ID {
	case ClaudeCode:
		return d.Command, claudeArgv(r), nil
	case Shell:
		// -l gives a login shell, so /etc/profile and the image's PATH are in
		// place. Nothing else: a terminal takes no options from this request,
		// which validatePrompt and friends have already enforced.
		return d.Command, []string{"-l"}, nil
	default:
		// Unreachable while Available() is true only for the two above. It is a
		// loud failure rather than a silent empty argv so that enabling a
		// harness in the catalogue without writing its builder is caught by the
		// first request instead of by a container that starts and exits.
		return "", nil, fmt.Errorf("%w: %q is available but has no argv builder", ErrUnavailable, d.ID)
	}
}

func claudeArgv(r Request) []string {
	argv := make([]string, 0, 12)
	if r.Model != "" {
		argv = append(argv, "--model", r.Model)
	}
	if r.Effort != "" {
		argv = append(argv, "--effort", string(r.Effort))
	}
	if flag, ok := claudePermissionFlag[r.Permission]; ok {
		argv = append(argv, "--permission-mode", flag)
	}
	// A resume FORKS, and the fork is named. Evidence: `claude --help` 2.1.274,
	// run inside panacea-app/sandbox:dev, and the CLI's behaviour there:
	//
	//   --fork-session  "When resuming, create a new session ID instead of
	//                   reusing the original (use with --resume or --continue)"
	//   --session-id    "Use a specific session ID for the conversation (must
	//                   be a valid UUID)"
	//
	// Reusing the original id is wrong for this product. The cockpit offers
	// Resume on a session that is still running, so reuse puts two processes,
	// in two sandboxes, appending to one transcript through the shared store —
	// and the CLI's own guard against that (--bg "starts a copy and says so
	// when the session is already running") keeps its state in
	// ~/.claude/sessions, which no sandbox shares, so it cannot see the other
	// one. A fork gives every cockpit session a transcript of its own.
	//
	// A bare fork gets an id only the CLI's output would reveal, so the fork is
	// named with --session-id. The CLI accepts that pairing and no other:
	// `--resume A --session-id B` without --fork-session fails with "--session-id
	// can only be used with --continue or --resume if --fork-session is also
	// specified", and --session-id naming a conversation that already exists
	// fails with "Session ID <id> is already in use" — which is why
	// validateResume requires Conversation and Resume to differ.
	if r.Resume != "" {
		argv = append(argv, "--resume", r.Resume, "--fork-session")
	}
	if r.Conversation != "" {
		argv = append(argv, "--session-id", r.Conversation)
	}
	if r.OpeningPrompt != "" {
		// Usage: claude [options] [command] [prompt]. Last, and whole.
		argv = append(argv, r.OpeningPrompt)
	}
	return argv
}

// The five validators share one rule: an unset field is not an option, so ""
// always passes. Anything else must be something the descriptor advertises,
// because the descriptor is what the picker rendered and the two must agree.

func validateModel(d Descriptor, model string) error {
	if model == "" {
		return nil
	}
	if len(d.Models) == 0 {
		return fmt.Errorf("%w: %q does not accept a model", ErrUnsupportedOption, d.ID)
	}
	if d.freeTextModel {
		return nil
	}
	for _, m := range d.Models {
		if m.ID == model {
			return nil
		}
	}
	return fmt.Errorf("%w: %q does not offer the model %q", ErrUnknownModel, d.ID, model)
}

func validateEffort(d Descriptor, effort Effort) error {
	if effort == "" {
		return nil
	}
	if len(d.Efforts) == 0 {
		// docs/FUNCTIONAL-SPEC.md line 27: effort is Claude Code only.
		return fmt.Errorf("%w: %q does not accept an effort level", ErrUnsupportedOption, d.ID)
	}
	for _, e := range d.Efforts {
		if e == effort {
			return nil
		}
	}
	return fmt.Errorf("%w: %q does not offer the effort %q", ErrUnsupportedOption, d.ID, effort)
}

func validatePermission(d Descriptor, mode PermissionMode) error {
	// "ask" is the absence of a flag on every harness, including a shell,
	// which is why it passes before the advertised-modes check.
	if mode == "" || mode == PermissionAsk {
		return nil
	}
	if len(d.PermissionModes) == 0 {
		return fmt.Errorf("%w: %q does not accept a permission mode", ErrUnsupportedOption, d.ID)
	}
	for _, m := range d.PermissionModes {
		if m == mode {
			return nil
		}
	}
	return fmt.Errorf("%w: %q does not offer the permission mode %q", ErrUnsupportedOption, d.ID, mode)
}

// conversationPattern is "a valid UUID" in the only spelling ids.UUID mints.
// Anything else reaching here came from a record that was edited or corrupted,
// and the CLI would reject it inside a terminal instead of here.
var conversationPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// IsConversationID is whether id is a conversation id in the one spelling
// ids.UUID mints — the check Build makes. It is exported for the places a
// record's id is joined onto a path in the conversation store, which must
// never join anything else: the look at whether a transcript was written
// (sessionTranscriptState, for resume and restart), and Forget, which deletes
// the transcript named after it (OD-10).
func IsConversationID(id string) bool { return conversationPattern.MatchString(id) }

// validateResume checks the two conversation ids together, because their
// rules are joint. The resume check comes first and keeps its wording: it is
// the one of these a client can actually cause, by asking a shell to resume.
// Everything after it is ErrConversationID — both ids come from the session
// record, never from a request body.
func validateResume(d Descriptor, resume, conversation string) error {
	if resume == "" && conversation == "" {
		return nil
	}
	if !d.SupportsResume {
		if resume != "" {
			return fmt.Errorf("%w: %q cannot resume a prior session", ErrUnsupportedOption, d.ID)
		}
		return fmt.Errorf("%w: %q has no conversation to name", ErrUnsupportedOption, d.ID)
	}
	if conversation != "" && !conversationPattern.MatchString(conversation) {
		return fmt.Errorf("%w: %q is not a UUID", ErrConversationID, conversation)
	}
	if resume == "" {
		return nil
	}
	if !conversationPattern.MatchString(resume) {
		return fmt.Errorf("%w: %q is not a UUID, so it is not a conversation the CLI can resume", ErrConversationID, resume)
	}
	if conversation == "" {
		return fmt.Errorf("%w: a resume needs the id its fork will carry, or the fork gets one nobody can learn without reading the CLI's output", ErrConversationID)
	}
	if conversation == resume {
		return fmt.Errorf("%w: the fork of %q needs a new id; the CLI refuses a --session-id that is already in use", ErrConversationID, resume)
	}
	return nil
}

func validatePrompt(d Descriptor, prompt string) error {
	if prompt == "" {
		return nil
	}
	if !d.SupportsOpeningPrompt {
		// A shell has no way to receive a sentence that is not "execute this
		// as a command", and silently executing what the user typed into a
		// prompt box is a footgun.
		return fmt.Errorf("%w: %q does not accept an opening prompt", ErrUnsupportedOption, d.ID)
	}
	return nil
}
