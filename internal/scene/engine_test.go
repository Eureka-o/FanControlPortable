package scene

import (
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestEngineRevertLastRetriesRuleAndBaseline(t *testing.T) {
	rules := []types.SceneRule{
		{Name: "Game", Match: "game.exe", Enabled: true, Gear: 4, RgbMode: 2},
	}

	var eng Engine
	act, changed := eng.Update(rules, "game.exe")
	if !changed || act.RuleIndex != 0 || act.Gear != 4 {
		t.Fatalf("first match = (%+v, %v), want rule 0", act, changed)
	}
	eng.RevertLast()
	actRetry, changedRetry := eng.Update(rules, "game.exe")
	if !changedRetry || actRetry.RuleIndex != 0 {
		t.Fatalf("retried match after RevertLast = (%+v, %v), want rule 0", actRetry, changedRetry)
	}

	actBase, changedBase := eng.Update(rules, "desktop.exe")
	if !changedBase || actBase.RuleIndex != ActiveBaseline {
		t.Fatalf("baseline transition = (%+v, %v), want ActiveBaseline", actBase, changedBase)
	}
	eng.RevertLast()
	actBaseRetry, changedBaseRetry := eng.Update(rules, "desktop.exe")
	if !changedBaseRetry || actBaseRetry.RuleIndex != ActiveBaseline {
		t.Fatalf("retried baseline after RevertLast = (%+v, %v), want ActiveBaseline", actBaseRetry, changedBaseRetry)
	}
}
