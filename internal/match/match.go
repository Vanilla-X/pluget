package match

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode"
)

// ArtifactMatches reports whether name matches pattern.
// Empty pattern matches everything. Patterns use path.Match semantics (* and ?).
func ArtifactMatches(pattern, name string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	ok, err := path.Match(pattern, name)
	return err == nil && ok
}

// VersionMatches reports whether candidate satisfies the configured constraint.
// Empty or "*" matches any version. Strings containing Maven range markers are
// parsed as Maven version ranges; otherwise exact equality (after NormalizeTag).
func VersionMatches(constraint, candidate string) bool {
	constraint = strings.TrimSpace(constraint)
	candidate = strings.TrimSpace(candidate)
	if constraint == "" || constraint == "*" {
		return true
	}
	if looksLikeRange(constraint) {
		ok, err := MatchRange(constraint, candidate)
		return err == nil && ok
	}
	return NormalizeTag(constraint) == NormalizeTag(candidate)
}

// NormalizeTag strips a leading "v" for tag comparison.
func NormalizeTag(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == 'v' || v[0] == 'V') && unicode.IsDigit(rune(v[1])) {
		return v[1:]
	}
	return v
}

func looksLikeRange(s string) bool {
	return strings.ContainsAny(s, "[(,)]")
}

// MatchRange evaluates a Maven version range expression against candidate.
// See https://maven.apache.org/enforcer/enforcer-rules/versionRanges.html
func MatchRange(expr, candidate string) (bool, error) {
	ranges, err := parseRanges(expr)
	if err != nil {
		return false, err
	}
	cv, err := ParseVersion(candidate)
	if err != nil {
		return false, err
	}
	for _, r := range ranges {
		if r.contains(cv) {
			return true, nil
		}
	}
	return false, nil
}

type versionRange struct {
	lower        *Version
	upper        *Version
	includeLower bool
	includeUpper bool
}

func (r versionRange) contains(v *Version) bool {
	if r.lower != nil {
		cmp := Compare(v, r.lower)
		if r.includeLower {
			if cmp < 0 {
				return false
			}
		} else if cmp <= 0 {
			return false
		}
	}
	if r.upper != nil {
		cmp := Compare(v, r.upper)
		if r.includeUpper {
			if cmp > 0 {
				return false
			}
		} else if cmp >= 0 {
			return false
		}
	}
	return true
}

func parseRanges(expr string) ([]versionRange, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty version range")
	}

	// Soft requirement: a single version without brackets means >= that version
	// per Maven "recommended" semantics used loosely; for our tool treat as exact
	// unless it has range chars (handled by caller). Soft "1.0" alone isn't a range.
	var ranges []versionRange
	rest := expr
	for strings.TrimSpace(rest) != "" {
		rest = strings.TrimSpace(rest)
		if rest[0] != '[' && rest[0] != '(' {
			return nil, fmt.Errorf("invalid version range %q", expr)
		}
		end := strings.IndexAny(rest, "])")
		if end < 0 {
			return nil, fmt.Errorf("unclosed version range %q", expr)
		}
		chunk := rest[:end+1]
		r, err := parseOneRange(chunk)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
		rest = strings.TrimSpace(rest[end+1:])
		if rest == "" {
			break
		}
		if rest[0] == ',' {
			rest = rest[1:]
			continue
		}
		return nil, fmt.Errorf("invalid version range separator in %q", expr)
	}
	return ranges, nil
}

func parseOneRange(s string) (versionRange, error) {
	if len(s) < 2 {
		return versionRange{}, fmt.Errorf("invalid range %q", s)
	}
	lowerInclusive := s[0] == '['
	upperInclusive := s[len(s)-1] == ']'
	if !lowerInclusive && s[0] != '(' {
		return versionRange{}, fmt.Errorf("invalid range start in %q", s)
	}
	if !upperInclusive && s[len(s)-1] != ')' {
		return versionRange{}, fmt.Errorf("invalid range end in %q", s)
	}
	inner := s[1 : len(s)-1]
	parts := strings.SplitN(inner, ",", 2)
	var r versionRange
	r.includeLower = lowerInclusive
	r.includeUpper = upperInclusive

	if len(parts) == 1 {
		// [1.0] exact
		v, err := ParseVersion(strings.TrimSpace(parts[0]))
		if err != nil {
			return versionRange{}, err
		}
		r.lower = v
		r.upper = v
		r.includeLower = true
		r.includeUpper = true
		return r, nil
	}

	low := strings.TrimSpace(parts[0])
	high := strings.TrimSpace(parts[1])
	if low != "" {
		v, err := ParseVersion(low)
		if err != nil {
			return versionRange{}, err
		}
		r.lower = v
	}
	if high != "" {
		v, err := ParseVersion(high)
		if err != nil {
			return versionRange{}, err
		}
		r.upper = v
	}
	return r, nil
}

// Version is a Maven-style version for comparison.
type Version struct {
	raw      string
	items    []item
}

type itemKind int

const (
	itemInt itemKind = iota
	itemString
	itemList
)

type item struct {
	kind itemKind
	i    int
	s    string
	list []item
}

// ParseVersion parses a Maven-compatible version string.
func ParseVersion(s string) (*Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty version")
	}
	s = NormalizeTag(s)
	items, err := parseItems(strings.ToLower(s))
	if err != nil {
		return nil, err
	}
	return &Version{raw: s, items: items}, nil
}

func parseItems(s string) ([]item, error) {
	// Split on '.' and '-' treating them as separators, similar to Maven ComparableVersion.
	var items []item
	var buf strings.Builder
	flush := func(isDigit bool) {
		if buf.Len() == 0 {
			return
		}
		part := buf.String()
		buf.Reset()
		if isDigit {
			n, _ := strconv.Atoi(part)
			items = append(items, item{kind: itemInt, i: n})
		} else {
			items = append(items, item{kind: itemString, s: qualify(part)})
		}
	}
	isDigit := false
	started := false
	for _, r := range s {
		if r == '.' || r == '-' || r == '_' {
			if started {
				flush(isDigit)
			}
			started = false
			continue
		}
		digit := unicode.IsDigit(r)
		if started && digit != isDigit {
			flush(isDigit)
			started = false
		}
		if !started {
			isDigit = digit
			started = true
		}
		buf.WriteRune(r)
	}
	if started {
		flush(isDigit)
	}
	return normalizeItems(items), nil
}

func qualify(s string) string {
	switch s {
	case "alpha", "a":
		return "alpha"
	case "beta", "b":
		return "beta"
	case "milestone", "m":
		return "milestone"
	case "rc", "cr":
		return "rc"
	case "snapshot":
		return "snapshot"
	case "ga", "final", "release", "":
		return ""
	case "sp":
		return "sp"
	default:
		return s
	}
}

func normalizeItems(items []item) []item {
	// Trim trailing zero integers and empty strings from the end.
	for len(items) > 0 {
		last := items[len(items)-1]
		if last.kind == itemInt && last.i == 0 {
			items = items[:len(items)-1]
			continue
		}
		if last.kind == itemString && last.s == "" {
			items = items[:len(items)-1]
			continue
		}
		break
	}
	return items
}

// Compare returns -1 if a<b, 0 if equal, 1 if a>b (Maven order).
func Compare(a, b *Version) int {
	return compareItems(a.items, b.items)
}

func compareItems(a, b []item) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		ai := item{kind: itemInt, i: 0}
		bi := item{kind: itemInt, i: 0}
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if c := compareItem(ai, bi); c != 0 {
			return c
		}
	}
	return 0
}

func compareItem(a, b item) int {
	// null / missing handled as 0 int above.
	// Ordering: int < string < list roughly per Maven; simplify:
	// integers vs integers, strings vs strings; mixed: string < int when comparing qualifier vs number
	if a.kind == itemInt && b.kind == itemInt {
		switch {
		case a.i < b.i:
			return -1
		case a.i > b.i:
			return 1
		default:
			return 0
		}
	}
	if a.kind == itemString && b.kind == itemString {
		return compareQualifier(a.s, b.s)
	}
	// string qualifier is less than a number (1.0-alpha < 1.0)
	if a.kind == itemString && b.kind == itemInt {
		return -1
	}
	if a.kind == itemInt && b.kind == itemString {
		return 1
	}
	return 0
}

func compareQualifier(a, b string) int {
	qa := qualifierRank(a)
	qb := qualifierRank(b)
	if qa != qb {
		if qa < qb {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

func qualifierRank(q string) int {
	switch q {
	case "alpha":
		return 1
	case "beta":
		return 2
	case "milestone":
		return 3
	case "rc":
		return 4
	case "snapshot":
		return 5
	case "":
		return 6
	case "sp":
		return 7
	default:
		return 0
	}
}

// MaxMatching returns the highest version (Maven order) among candidates that match constraint.
// Candidates should be unique version strings. Returns "" if none match.
func MaxMatching(constraint string, candidates []string) string {
	var best string
	var bestV *Version
	for _, c := range candidates {
		if !VersionMatches(constraint, c) {
			continue
		}
		v, err := ParseVersion(c)
		if err != nil {
			continue
		}
		if bestV == nil || Compare(v, bestV) > 0 {
			best = c
			bestV = v
		}
	}
	return best
}
