package worldrepo

import "sync"

var developmentInteractiveTitleFirst sync.Map
var developmentInteractiveTitleFirstPlanningLocks sync.Map

// EnableDevelopmentInteractiveTitleFirstPoC makes the ordinary dial-up path for
// the development materialization host use the same conversation-view +
// title-first planning rules as the isolated fresh Lab, but marks this repository
// as interactive so expensive work can be kept off the terminal's critical path.
//
// Only the materialization-demo host program calls the development-only
// Materialization* repository methods, so enabling this on the shared runtime
// repository does not replace historical host-program behavior for other BBSes.
func (r *Repository) EnableDevelopmentInteractiveTitleFirstPoC() {
	developmentInteractiveTitleFirst.Store(r, true)
	r.EnableDevelopmentConversationViewPoC()
	r.SetDevelopmentConversationShellLimit(12)
	r.EnableDevelopmentTitleFirstPoC(nil)
}

func developmentInteractiveTitleFirstEnabled(r *Repository) bool {
	_, ok := developmentInteractiveTitleFirst.Load(r)
	return ok
}

func developmentInteractiveTitleFirstPlanningMutex(r *Repository) *sync.Mutex {
	value, _ := developmentInteractiveTitleFirstPlanningLocks.LoadOrStore(r, &sync.Mutex{})
	return value.(*sync.Mutex)
}
