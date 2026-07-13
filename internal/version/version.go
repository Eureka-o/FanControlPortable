package version

import (
	"strconv"
	"strings"
)

// BuildVersion 在编译时通过 ldflags 注入版本号
// 示例: go build -ldflags "-X github.com/TIANLI0/THRM/internal/version.BuildVersion=2.1.0"
var BuildVersion = "dev"

// Get 返回应用版本号
// 优先使用编译时注入的版本号，如果未注入则返回 "dev"
func Get() string {
	if v := strings.TrimSpace(BuildVersion); v != "" {
		return v
	}
	return "dev"
}

type parsedVersion struct {
	parts      []int
	prerelease []string
}

// IsNewer reports whether candidate should replace current.
func IsNewer(current, candidate string) bool {
	current = normalize(current)
	candidate = normalize(candidate)
	if current == "" || candidate == "" || current == candidate {
		return false
	}

	currentNightly, currentIsNightly := parseNightly(current)
	candidateNightly, candidateIsNightly := parseNightly(candidate)
	if currentIsNightly && candidateIsNightly {
		return candidateNightly > currentNightly
	}

	currentVersion, currentOK := parse(current)
	candidateVersion, candidateOK := parse(candidate)
	if !currentOK || !candidateOK {
		return true
	}

	length := max(len(currentVersion.parts), len(candidateVersion.parts))
	for index := 0; index < length; index++ {
		currentPart := partAt(currentVersion.parts, index)
		candidatePart := partAt(candidateVersion.parts, index)
		if currentPart != candidatePart {
			return candidatePart > currentPart
		}
	}

	if len(currentVersion.prerelease) == 0 || len(candidateVersion.prerelease) == 0 {
		return len(currentVersion.prerelease) > 0 && len(candidateVersion.prerelease) == 0
	}
	return prereleaseIsNewer(currentVersion.prerelease, candidateVersion.prerelease)
}

func normalize(value string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "v")
}

func parseNightly(value string) (int, bool) {
	suffix, ok := strings.CutPrefix(value, "nightly")
	if !ok {
		return 0, false
	}
	suffix = strings.TrimPrefix(strings.TrimPrefix(suffix, "-"), ".")
	if len(suffix) != 8 {
		return 0, false
	}
	date, err := strconv.Atoi(suffix)
	return date, err == nil
}

func parse(value string) (parsedVersion, bool) {
	value, _, _ = strings.Cut(value, "+")
	base, suffix, hasPrerelease := strings.Cut(value, "-")
	baseParts := strings.Split(base, ".")
	if len(baseParts) == 0 || len(baseParts) > 4 {
		return parsedVersion{}, false
	}
	parsed := parsedVersion{parts: make([]int, len(baseParts))}
	for index, part := range baseParts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return parsedVersion{}, false
		}
		parsed.parts[index] = value
	}
	if hasPrerelease && suffix != "" {
		parsed.prerelease = strings.Split(suffix, ".")
	}
	return parsed, true
}

func prereleaseIsNewer(current, candidate []string) bool {
	length := max(len(current), len(candidate))
	for index := 0; index < length; index++ {
		if index >= len(current) {
			return true
		}
		if index >= len(candidate) {
			return false
		}
		if current[index] == candidate[index] {
			continue
		}

		currentNumeric := isNumeric(current[index])
		candidateNumeric := isNumeric(candidate[index])
		if currentNumeric && candidateNumeric {
			return compareNumeric(candidate[index], current[index]) > 0
		}
		if currentNumeric != candidateNumeric {
			return currentNumeric
		}
		return candidate[index] > current[index]
	}
	return false
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func compareNumeric(left, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if len(left) != len(right) {
		return len(left) - len(right)
	}
	return strings.Compare(left, right)
}

func partAt(parts []int, index int) int {
	if index < len(parts) {
		return parts[index]
	}
	return 0
}
