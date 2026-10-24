// Package scene 实现「情景」：按前台进程自动施加散热/灯效设置。
package scene

import (
	"strings"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

// ActiveBaseline 表示"没有任何情景规则匹配"。
const ActiveBaseline = -1

// Action 描述需要施加的动作。
type Action struct {
	// RuleIndex 生效规则的索引；ActiveBaseline 表示回落到基准配置。
	RuleIndex int

	// Gear / RgbMode 目标值；0 表示该项不改变。
	Gear    int
	RgbMode int

	// Foreground 触发本次变化的进程名（便于日志与界面显示）。
	Foreground string
}

// NormalizeProcess 把进程名统一成可比较的形式：
// 去空白、去路径、去 `.exe`、转小写。
func NormalizeProcess(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return ""
	}
	// 去掉可能的路径前缀（"C:\a\b\game.exe" -> "game.exe"）
	if i := strings.LastIndexAny(s, `\/`); i >= 0 {
		s = s[i+1:]
	}
	s = strings.ToLower(s)
	s = strings.TrimSuffix(s, ".exe")
	return s
}

// MatchRule 返回命中的规则索引；没有命中返回 ActiveBaseline。
//
// 规则按数组顺序匹配，先到先得——更具体的规则可以放在前面覆盖后面的。
func MatchRule(rules []types.SceneRule, foreground string) int {
	who := NormalizeProcess(foreground)
	if who == "" {
		return ActiveBaseline
	}
	for i, r := range rules {
		if !r.Enabled {
			continue
		}
		if NormalizeProcess(r.Match) == who {
			return i
		}
	}
	return ActiveBaseline
}

// Engine 把"前台进程"翻译成"是否需要施加新动作"。
//
// 零值可用；它不是并发安全的，调用方负责串行化（项目里由单个后台循环驱动）。
type Engine struct {
	// lastActive 上一次生效的规则索引；ActiveBaseline 表示基准。
	lastActive int
	// primed 表示是否已经有过一次判定。
	// 首次判定即使结果是基准也不产生动作：启动时重放一次基准没有意义，还会白写一轮设备。
	primed bool
}

// Update 用当前前台进程名推进一次判定。
//
// 返回 (action, true) 表示需要施加该动作；(_, false) 表示不需要做任何事。
func (e *Engine) Update(rules []types.SceneRule, foreground string) (Action, bool) {
	active := MatchRule(rules, foreground)

	if !e.primed {
		// 首次判定：基准不产生动作（否则白写一轮设备）；命中规则必须产生动作，
		// 因为启动时那个进程可能已经在前台。
		e.primed = true
		e.lastActive = active
		if active == ActiveBaseline {
			return Action{}, false
		}
		return e.buildAction(rules, active, foreground), true
	}

	if active == e.lastActive {
		return Action{}, false
	}
	e.lastActive = active
	return e.buildAction(rules, active, foreground), true
}

// buildAction 把"生效规则索引"翻译成动作。
// 基准（ActiveBaseline）返回零值动作，由调用方施加自己的基准配置。
func (e *Engine) buildAction(rules []types.SceneRule, active int, foreground string) Action {
	act := Action{RuleIndex: active, Foreground: NormalizeProcess(foreground)}
	if active >= 0 && active < len(rules) {
		act.Gear = rules[active].Gear
		act.RgbMode = rules[active].RgbMode
	}
	return act
}

// Reset 清空内部状态（例如规则表被外部整体替换后调用），
// 使下一次 Update 重新做一次完整判定。
func (e *Engine) Reset() {
	e.lastActive = ActiveBaseline
	e.primed = false
}
