package registry

// Positive private classification survives a missing owner discriminator.
// Generic session APIs cannot use a personal utility as an interactive worker.
func (s Session) IsPersonalAssistant() bool {
	return s.PersonalOwner != "" || s.ContextRef == "luci:personal"
}
