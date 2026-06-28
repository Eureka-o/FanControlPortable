package deviceprofileexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

type ParsedState struct {
	CurrentSpeed int
	HasCurrent   bool
	TargetSpeed  int
	HasTarget    bool
	WorkMode     string
}

// completeStateTarget keeps current speed honest: fallback values may fill target speed, never current speed.
func completeStateTarget(state *ParsedState, fallbackTarget int) bool {
	if state == nil {
		return false
	}
	if !state.HasTarget {
		if fallbackTarget > 0 {
			state.TargetSpeed = fallbackTarget
			state.HasTarget = true
		} else if state.HasCurrent {
			state.TargetSpeed = state.CurrentSpeed
			state.HasTarget = true
		}
	}
	return state.HasCurrent || state.HasTarget
}

type CompiledResponseParsers struct {
	parsers []compiledResponseParser
}

type compiledResponseParser struct {
	source    types.DeviceResponseParser
	role      string
	valueType string
	jsonParts []string
	offset    int
	regex     *regexp.Regexp
}

func CompileResponseParsers(parsers []types.DeviceResponseParser) (CompiledResponseParsers, error) {
	compiled := CompiledResponseParsers{
		parsers: make([]compiledResponseParser, 0, len(parsers)),
	}
	for _, parser := range parsers {
		valueType := normalizeKey(parser.Type)
		item := compiledResponseParser{
			source:    parser,
			role:      parserRole(parser.Name),
			valueType: valueType,
		}
		switch valueType {
		case "jsonpath":
			item.jsonParts = splitJSONPath(parser.Expression)
		case "byteoffset":
			offset, err := strconv.Atoi(strings.TrimSpace(parser.Expression))
			if err != nil {
				return compiled, fmt.Errorf("byte offset parser expression must be an integer")
			}
			item.offset = offset
		case "regex":
			re, err := regexp.Compile(parser.Expression)
			if err != nil {
				return compiled, fmt.Errorf("regex response parser expression is invalid: %w", err)
			}
			item.regex = re
		case "plain":
		default:
			return compiled, fmt.Errorf("unsupported response parser type %q", parser.Type)
		}
		compiled.parsers = append(compiled.parsers, item)
	}
	return compiled, nil
}

func ParseState(body []byte, parsers []types.DeviceResponseParser) (ParsedState, error) {
	compiled, err := CompileResponseParsers(parsers)
	if err != nil {
		return ParsedState{}, err
	}
	return compiled.Parse(body)
}

func (c CompiledResponseParsers) Parse(body []byte) (ParsedState, error) {
	var parsed ParsedState
	if len(bytes.TrimSpace(body)) == 0 {
		return parsed, nil
	}

	if state, ok := parseDefaultWiFiState(body); ok {
		parsed = state
	}

	firstNumeric := 0
	hasFirstNumeric := false

	var rawJSON any
	jsonParsed := false
	var jsonErr error
	for _, parser := range c.parsers {
		value, ok, err := parser.parse(body, &rawJSON, &jsonParsed, &jsonErr)
		if err != nil {
			return parsed, err
		}
		if !ok {
			continue
		}
		switch parser.role {
		case "target":
			parsed.TargetSpeed = value
			parsed.HasTarget = true
		case "current":
			parsed.CurrentSpeed = value
			parsed.HasCurrent = true
		default:
			if !hasFirstNumeric {
				firstNumeric = value
				hasFirstNumeric = true
			}
		}
	}

	if !parsed.HasCurrent && hasFirstNumeric {
		parsed.CurrentSpeed = firstNumeric
		parsed.HasCurrent = true
	}
	if !parsed.HasTarget && parsed.HasCurrent {
		parsed.TargetSpeed = parsed.CurrentSpeed
		parsed.HasTarget = true
	}
	return parsed, nil
}

func parseDefaultWiFiState(body []byte) (ParsedState, bool) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ParsedState{}, false
	}
	if !looksLikeDefaultWiFiState(raw) {
		return ParsedState{}, false
	}

	state := ParsedState{}
	if speed, ok := numberFromKeys(raw, "currentSpeed", "currentRpm", "fanSpeed", "speed"); ok {
		state.CurrentSpeed = speed
		state.HasCurrent = true
	}
	if target, ok := numberFromKeys(raw, "wifiTargetSpeed", "targetSpeed", "targetRpm", "speed"); ok {
		state.TargetSpeed = target
		state.HasTarget = true
	}
	if !state.HasTarget && state.HasCurrent {
		state.TargetSpeed = state.CurrentSpeed
		state.HasTarget = true
	}
	if mode, ok := stringFromAny(raw["controlMode"]); ok {
		state.WorkMode = mode
	} else if mode, ok := stringFromAny(raw["mode"]); ok {
		state.WorkMode = mode
	}
	return state, state.HasCurrent || state.HasTarget
}

func looksLikeDefaultWiFiState(raw map[string]any) bool {
	hasCurrent := hasNumberKey(raw, "fanSpeed", "currentSpeed", "currentRpm")
	hasGenericSpeed := hasNumberKey(raw, "speed")
	hasTarget := hasNumberKey(raw, "wifiTargetSpeed", "targetSpeed", "targetRpm")
	if !hasCurrent && !hasGenericSpeed && !hasTarget {
		return false
	}
	if hasCurrent || hasTarget {
		return true
	}
	if _, ok := boolFromAny(raw["wifiControl"]); ok {
		return true
	}
	if hasNumberKey(raw, "temperature", "power") {
		return true
	}
	if _, ok := boolFromAny(raw["power"]); ok {
		return true
	}
	return hasModeLikeKey(raw, "controlMode", "mode")
}

func hasNumberKey(raw map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := numberFromAny(raw[key]); ok {
			return true
		}
	}
	return false
}

func hasModeLikeKey(raw map[string]any, keys ...string) bool {
	for _, key := range keys {
		mode, ok := stringFromAny(raw[key])
		if !ok {
			continue
		}
		mode = strings.ToLower(strings.TrimSpace(mode))
		if mode == "manual" || mode == "wifi" || mode == "software" || strings.Contains(mode, "auto") {
			return true
		}
	}
	return false
}

func boolFromAny(value any) (bool, bool) {
	switch v := value.(type) {
	case nil:
		return false, false
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "on", "yes", "auto", "wifi", "software":
			return true, true
		case "false", "0", "off", "no", "manual":
			return false, true
		}
	case float64:
		return v != 0, true
	}
	return false, false
}

func numberFromKeys(raw map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := numberFromAny(raw[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func (p compiledResponseParser) parse(body []byte, rawJSON *any, jsonParsed *bool, jsonErr *error) (int, bool, error) {
	switch p.valueType {
	case "jsonpath":
		return p.parseJSONPathValue(body, rawJSON, jsonParsed, jsonErr)
	case "byteoffset":
		return p.parseByteOffsetValue(body)
	case "regex":
		return p.parseRegexValue(body)
	case "plain":
		return 0, false, nil
	default:
		return 0, false, fmt.Errorf("unsupported response parser type %q", p.source.Type)
	}
}

func splitJSONPath(expression string) []string {
	expression = strings.TrimSpace(expression)
	if strings.HasPrefix(expression, "$.") {
		expression = strings.TrimPrefix(expression, "$.")
	}
	if expression == "" || expression == "$" {
		return nil
	}

	parts := strings.Split(expression, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (p compiledResponseParser) parseJSONPathValue(body []byte, rawJSON *any, jsonParsed *bool, jsonErr *error) (int, bool, error) {
	if len(p.jsonParts) == 0 {
		return 0, false, nil
	}

	if !*jsonParsed {
		*jsonParsed = true
		*jsonErr = json.Unmarshal(body, rawJSON)
	}
	if *jsonErr != nil {
		return 0, false, fmt.Errorf("json response parser could not parse JSON: %w", *jsonErr)
	}
	current := *rawJSON
	for _, part := range p.jsonParts {
		obj, ok := current.(map[string]any)
		if !ok {
			return 0, false, nil
		}
		current, ok = obj[part]
		if !ok {
			return 0, false, nil
		}
	}
	value, ok := numberFromAny(current)
	return value, ok, nil
}

func (p compiledResponseParser) parseByteOffsetValue(body []byte) (int, bool, error) {
	if p.offset < 0 || p.offset >= len(body) {
		return 0, false, nil
	}
	return int(body[p.offset]), true, nil
}

func (p compiledResponseParser) parseRegexValue(body []byte) (int, bool, error) {
	if p.regex == nil {
		return 0, false, nil
	}
	matches := p.regex.FindStringSubmatch(string(body))
	if len(matches) == 0 {
		return 0, false, nil
	}
	value := matches[0]
	if len(matches) > 1 {
		value = matches[1]
	}
	parsed, ok := numberFromAny(value)
	return parsed, ok, nil
}

func parserRole(name string) string {
	key := normalizeKey(name)
	switch {
	case strings.Contains(key, "target"):
		return "target"
	case strings.Contains(key, "current"), strings.Contains(key, "speed"), strings.Contains(key, "state"):
		return "current"
	default:
		return ""
	}
}

func numberFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case float64:
		return int(v + 0.5), true
	case float32:
		return int(v + 0.5), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		return int(i), err == nil
	case string:
		v = strings.TrimSpace(strings.TrimSuffix(v, "%"))
		if v == "" {
			return 0, false
		}
		i, err := strconv.Atoi(v)
		return i, err == nil
	default:
		return 0, false
	}
}

func stringFromAny(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	if s, ok := value.(string); ok {
		s = strings.TrimSpace(s)
		return s, s != ""
	}
	return fmt.Sprint(value), true
}
