package coreapp

import (
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/smartcontrol"
)

// Kept as a source-compatible alias for diagnostics/tests; the decision
// contract lives in internal/smartcontrol.
type smartControlDecisionSnapshot = smartcontrol.Decision

func (a *CoreApp) setSmartControlDecision(snapshot smartcontrol.Decision) {
	if a == nil {
		return
	}
	if snapshot.Timestamp == "" {
		snapshot.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	a.smartControlDecisionMu.Lock()
	a.smartControlDecision = snapshot
	a.smartControlDecisionMu.Unlock()
}

func (a *CoreApp) getSmartControlDecision() smartcontrol.Decision {
	if a == nil {
		return smartcontrol.Decision{}
	}
	a.smartControlDecisionMu.RLock()
	snapshot := a.smartControlDecision
	a.smartControlDecisionMu.RUnlock()
	return snapshot
}
