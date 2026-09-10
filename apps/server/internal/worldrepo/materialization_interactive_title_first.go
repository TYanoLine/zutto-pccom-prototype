package worldrepo

// EnableDevelopmentInteractiveTitleFirstPoC makes the ordinary dial-up path for
// the development materialization host use the same conversation-view +
// title-first planning pipeline that the isolated fresh Lab exercises.
//
// Only the materialization-demo host program calls the development-only
// Materialization* repository methods, so enabling this on the shared runtime
// repository does not replace historical host-program behavior for other BBSes.
func (r *Repository) EnableDevelopmentInteractiveTitleFirstPoC() {
	r.EnableDevelopmentConversationViewPoC()
	r.SetDevelopmentConversationShellLimit(8)
	r.EnableDevelopmentTitleFirstPoC(nil)
}
