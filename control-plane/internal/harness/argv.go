package harness

import "fmt"

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
// effort, permission, resume, prompt, in that order — so a request with two
// faults fails the same way every run. That is not Request's declaration
// order: the struct declares OpeningPrompt before Resume, but Build checks
// resume before prompt, because this list is the contract's stated order,
// not an accident of the struct layout. On any error both return values are
// zero: a partial argv is how a rejected request becomes a started session.
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
	if err := validateResume(d, r.Resume); err != nil {
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
	argv := make([]string, 0, 9)
	if r.Model != "" {
		argv = append(argv, "--model", r.Model)
	}
	if r.Effort != "" {
		argv = append(argv, "--effort", string(r.Effort))
	}
	if flag, ok := claudePermissionFlag[r.Permission]; ok {
		argv = append(argv, "--permission-mode", flag)
	}
	if r.Resume != "" {
		argv = append(argv, "--resume", r.Resume)
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

func validateResume(d Descriptor, resume string) error {
	if resume == "" {
		return nil
	}
	if !d.SupportsResume {
		return fmt.Errorf("%w: %q cannot resume a prior session", ErrUnsupportedOption, d.ID)
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
