package worldrepo

import (
	"fmt"
	"strings"
	"sync"
)

type developmentPlanningKey struct {
	repo    *Repository
	hostID  string
	boardID string
}

var developmentPlanningUsage sync.Map
var developmentPlanningErrors sync.Map

func storeDevelopmentPlanningUsage(r *Repository, hostID, boardID string, usage GenerationUsage) {
	if usage.TotalTokens == 0 && usage.Model == "" {
		return
	}
	developmentPlanningUsage.Store(developmentPlanningKey{repo: r, hostID: hostID, boardID: boardID}, usage)
}

func storeDevelopmentPlanningError(r *Repository, hostID, boardID string, err error) {
	if err == nil {
		return
	}
	developmentPlanningErrors.Store(developmentPlanningKey{repo: r, hostID: hostID, boardID: boardID}, strings.Join(strings.Fields(err.Error()), " "))
}

func clearDevelopmentPlanningError(r *Repository, hostID, boardID string) {
	developmentPlanningErrors.Delete(developmentPlanningKey{repo: r, hostID: hostID, boardID: boardID})
}

func (r *Repository) MaterializationPlanningDiagnostic(hostID, boardID string) string {
	key := developmentPlanningKey{repo: r, hostID: hostID, boardID: boardID}
	parts := make([]string, 0, 3)
	if value, ok := developmentPlanningErrors.Load(key); ok {
		if message, ok := value.(string); ok && message != "" {
			parts = append(parts, "planning_error="+message)
		}
	}
	if value, ok := developmentPlanningUsage.Load(key); ok {
		if usage, ok := value.(GenerationUsage); ok {
			parts = append(parts, "planning_tokens="+formatGenerationUsage(usage))
		}
	}
	return strings.Join(parts, " ")
}

func clearDevelopmentPlanningTelemetry(r *Repository) {
	developmentPlanningUsage.Range(func(key, _ any) bool {
		if planningKey, ok := key.(developmentPlanningKey); ok && planningKey.repo == r {
			developmentPlanningUsage.Delete(key)
		}
		return true
	})
	developmentPlanningErrors.Range(func(key, _ any) bool {
		if planningKey, ok := key.(developmentPlanningKey); ok && planningKey.repo == r {
			developmentPlanningErrors.Delete(key)
		}
		return true
	})
}
