package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type wildcardType int

const (
	noneWildcard  wildcardType = iota
	majorWildcard wildcardType = 1
	minorWildcard wildcardType = 2
	patchWildcard wildcardType = 3
)

func wildcardTypefromInt(i int) wildcardType {
	switch i {
	case 1:
		return majorWildcard
	case 2:
		return minorWildcard
	case 3:
		return patchWildcard
	default:
		return noneWildcard
	}
}

type comparator func(Version, Version) bool

var (
	compEQ comparator = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) == 0
	}
	compNE = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) != 0
	}
	compGT = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) == 1
	}
	compGE = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) >= 0
	}
	compLT = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) == -1
	}
	compLE = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) <= 0
	}
)

type versionRange struct {
	v Version
	c comparator
}

// rangeFunc creates a Range from the given versionRange.
func (vr *versionRange) rangeFunc() Range {
	return Range(func(v Version) bool {
		return vr.c(v, vr.v)
	})
}

// Range represents a range of versions.
// A Range can be used to check if a Version satisfies it:
//
//     range, err := semver.ParseRange(">1.0.0 <2.0.0")
//     range(semver.MustParse("1.1.1") // returns true
type Range func(Version) bool

// OR combines the existing Range with another Range using logical OR.
func (rf Range) OR(f Range) Range {
	return Range(func(v Version) bool {
		return rf(v) || f(v)
	})
}

// AND combines the existing Range with another Range using logical AND.
func (rf Range) AND(f Range) Range {
	return Range(func(v Version) bool {
		return rf(v) && f(v)
	})
}

// ParseRange parses a range and returns a Range.
// If the range could not be parsed an error is returned.
//
// Valid ranges are:
//   - "<1.0.0"
//   - "<=1.0.0"
//   - ">1.0.0"
//   - ">=1.0.0"
//   - "1.0.0", "=1.0.0", "==1.0.0"
//   - "!1.0.0", "!=1.0.0"
//
// A Range can consist of multiple ranges separated by space:
// Ranges can be linked by logical AND:
//   - ">1.0.0 <2.0.0" would match between both ranges, so "1.1.1" and "1.8.7" but not "1.0.0" or "2.0.0"
//   - ">1.0.0 <3.0.0 !2.0.3-beta.2" would match every version between 1.0.0 and 3.0.0 except 2.0.3-beta.2
//
// Ranges can also be linked by logical OR:
//   - "<2.0.0 || >=3.0.0" would match "1.x.x" and "3.x.x" but not "2.x.x"
//
// AND has a higher precedence than OR. It's not possible to use brackets.
//
// Ranges can be combined by both AND and OR
//
//  - `>1.0.0 <2.0.0 || >3.0.0 !4.2.1` would match `1.2.3`, `1.9.9`, `3.1.1`, but not `4.2.1`, `2.1.1`
func ParseRange(s string) (Range, error) {
	// s = replaceStars(s)
	// parts := splitAndTrim(s)

	var expandedParts [][]string
	// split on boolean or ||
	orParts := regexp.MustCompile("\\s*\\|\\|\\s*").Split(s, -1)
	for _, part := range orParts {
		parsed := parseRange(part)
		if len(parsed) > 0 {
			expandedParts = append(expandedParts, parseRange(strings.TrimSpace(part)))
		}
	}

	// orParts, err := splitORParts(parts)
	// if err != nil {
	// 	return nil, err
	// }
	// expandedParts, err := expandWildcardVersion(orParts)
	// if err != nil {
	// 	return nil, err
	// }

	var orFn Range
	for _, p := range expandedParts {
		var andFn Range
		for _, ap := range p {
			opStr, vStr, err := splitComparatorVersion(ap)
			if err != nil {
				return nil, err
			}
			vr, err := buildVersionRange(opStr, vStr)
			if err != nil {
				return nil, fmt.Errorf("Could not parse Range %q: %s", ap, err)
			}
			rf := vr.rangeFunc()

			// Set function
			if andFn == nil {
				andFn = rf
			} else { // Combine with existing function
				andFn = andFn.AND(rf)
			}
		}
		if orFn == nil {
			orFn = andFn
		} else {
			orFn = orFn.OR(andFn)
		}

	}
	return orFn, nil
}

func parseRange(s string) []string {
	var out []string
	s = strings.TrimSpace(s)
	re := getRegex()

	// `1.2.3 - 1.2.4` => `>=1.2.3 <=1.2.4`
	s = hyphenReplace(re, s)

	// fmt.Println("hyphen replace", s)
	// `> 1.2.3 < 1.2.5` => `>1.2.3 <1.2.5`
	s = re["COMPARATORTRIM"].ReplaceAllString(s, "$1$2$3")
	// fmt.Println("comparator trim", s)
	// `~ 1.2.3` => `~1.2.3`
	s = re["TILDETRIM"].ReplaceAllString(s, "$1~")
	// `^ 1.2.3` => `^1.2.3
	s = re["CARETTRIM"].ReplaceAllString(s, "$1^")
	// normalize spaces
	s = strings.Join(regexp.MustCompile("\\s+").Split(s, -1), " ")

	// fmt.Println("post trim", s)
	// At this point, the range is completely trimmed and
	// ready to be split into comparators.
	for _, comp := range strings.Split(s, " ") {
		parsed := parseComparatorString(re, comp)
		out = append(out, parsed)
	}

	// join and split by spaces once more
	return regexp.MustCompile("\\s+").Split(strings.Join(out, " "), -1)
}

// comprised of xranges, tildes, stars, and gtlt's at this point.
// already replaced the hyphen ranges
// turn into a set of JUST comparators.
func parseComparatorString(re map[string]*regexp.Regexp, s string) string {
	s = replaceCarets(re, s)
	s = replaceTildes(re, s)
	s = replaceXRanges(re, s)
	s = replaceStars(re, s)
	return s
}

// 1.2 - 3.4.5 => >=1.2.0 <=3.4.5
// 1.2.3 - 3.4 => >=1.2.3 <3.5.0 Any 3.4.x will do
// 1.2 - 3.4 => >=1.2.0 <3.5.0
func hyphenReplace(re map[string]*regexp.Regexp, s string) string {
	// if we don't match for a hyphen range, return the string unchanged
	if !re["HYPHENRANGE"].MatchString(s) {
		return s
	}
	match := re["HYPHENRANGE"].FindStringSubmatch(s)

	from := match[1]
	fM := match[2]
	fm := match[3]
	fp := match[4]
	// fpr := match[5]
	// fb := match[6]

	to := match[7]
	tM := match[8]
	tm := match[9]
	tp := match[10]
	tpr := match[11]
	// tb := match[12]

	if isX(fM) {
		from = ""
	} else if isX(fm) {
		from = ">=" + fM + ".0.0"
	} else if isX(fp) {
		from = ">=" + fM + "." + fm + ".0"
	} else {
		from = ">=" + from
	}

	if isX(tM) {
		to = ""
	} else if isX(tm) {
		major, _ := strconv.Atoi(tM)
		to = "<" + strconv.Itoa(major+1) + ".0.0"
	} else if isX(tp) {
		minor, _ := strconv.Atoi(tm)
		to = "<" + tM + "." + strconv.Itoa(minor+1) + ".0"
	} else if len(tpr) > 0 {
		to = "<=" + tM + "." + tm + "." + tp + "-" + tpr
	} else {
		to = "<=" + to
	}

	return strings.TrimSpace(from + " " + to)
}

// ~, ~> --> * (any, kinda silly)
// ~2, ~2.x, ~2.x.x, ~>2, ~>2.x ~>2.x.x --> >=2.0.0 <3.0.0
// ~2.0, ~2.0.x, ~>2.0, ~>2.0.x --> >=2.0.0 <2.1.0
// ~1.2, ~1.2.x, ~>1.2, ~>1.2.x --> >=1.2.0 <1.3.0
// ~1.2.3, ~>1.2.3 --> >=1.2.3 <1.3.0
// ~1.2.0, ~>1.2.0 --> >=1.2.0 <1.3.0
func replaceTildes(re map[string]*regexp.Regexp, s string) string {
	var acc []string
	s = strings.TrimSpace(s)
	parts := regexp.MustCompile("\\s+").Split(s, -1)
	for _, p := range parts {
		acc = append(acc, replaceTilde(re, p))
	}
	return strings.Join(acc, " ")
}

func replaceTilde(re map[string]*regexp.Regexp, s string) string {
	// if we don't match for a hyphen range, return the string unchanged
	if !re["TILDE"].MatchString(s) {
		return s
	}
	ret := s
	match := re["TILDE"].FindStringSubmatch(s)

	M := match[1]
	m := match[2]
	p := match[3]
	pr := match[4]

	// parsed version numbers as ints
	// the regex ensures they are valid numbers
	major, _ := strconv.Atoi(M)
	minor, _ := strconv.Atoi(m)

	if isX(M) {
		ret = ""
	} else if isX(m) {
		ret = ">=" + M + ".0.0 <" + strconv.Itoa(major+1) + ".0.0"
	} else if isX(p) {
		// ~1.2 == >=1.2.0 <1.3.0
		ret = ">=" + M + "." + m + ".0 <" + M + "." + strconv.Itoa(minor+1) + ".0"
	} else if len(pr) > 0 {
		ret = ">=" + M + "." + m + "." + p + "-" + pr +
			" <" + M + "." + strconv.Itoa(minor+1) + ".0"
	} else {
		// ~1.2.3 == >=1.2.3 <1.3.0
		ret = ">=" + M + "." + m + "." + p +
			" <" + M + "." + strconv.Itoa(minor+1) + ".0"
	}

	return ret
}

// ^ --> * (any, kinda silly)
// ^2, ^2.x, ^2.x.x --> >=2.0.0 <3.0.0
// ^2.0, ^2.0.x --> >=2.0.0 <3.0.0
// ^1.2, ^1.2.x --> >=1.2.0 <2.0.0
// ^1.2.3 --> >=1.2.3 <2.0.0
// ^1.2.0 --> >=1.2.0 <2.0.0
func replaceCarets(re map[string]*regexp.Regexp, s string) string {
	var acc []string
	s = strings.TrimSpace(s)
	parts := regexp.MustCompile("\\s+").Split(s, -1)
	for _, p := range parts {
		acc = append(acc, replaceCaret(re, p))
	}
	return strings.Join(acc, " ")
}

func replaceCaret(re map[string]*regexp.Regexp, s string) string {
	// if we don't match for a hyphen range, return the string unchanged
	if !re["CARET"].MatchString(s) {
		return s
	}
	ret := s
	match := re["CARET"].FindStringSubmatch(s)

	M := match[1]
	m := match[2]
	p := match[3]
	pr := match[4]

	// parsed version numbers as ints
	// the regex ensures they are valid numbers
	major, _ := strconv.Atoi(M)
	minor, _ := strconv.Atoi(m)
	patch, _ := strconv.Atoi(p)

	if isX(M) {
		ret = ""
	} else if isX(m) {
		ret = ">=" + M + ".0.0 <" + strconv.Itoa(major+1) + ".0.0"
	} else if isX(p) {
		if M == "0" {
			ret = ">=" + M + "." + m + ".0 <" + M + "." + strconv.Itoa(minor+1) + ".0"
		} else {
			ret = ">=" + M + "." + m + ".0 <" + strconv.Itoa(major+1) + ".0.0"
		}
	} else if len(pr) > 0 {
		if M == "0" {
			if m == "0" {
				ret = ">=" + M + "." + m + "." + p + "-" + pr +
					" <" + M + "." + m + "." + strconv.Itoa(patch+1)
			} else {
				ret = ">=" + M + "." + m + "." + p + "-" + pr +
					" <" + M + "." + strconv.Itoa(minor+1) + ".0"
			}
		} else {
			ret = ">=" + M + "." + m + "." + p + "-" + pr +
				" <" + strconv.Itoa(major+1) + ".0.0"
		}
	} else {
		if M == "0" {
			if m == "0" {
				ret = ">=" + M + "." + m + "." + p +
					" <" + M + "." + m + "." + strconv.Itoa(patch+1)
			} else {
				ret = ">=" + M + "." + m + "." + p +
					" <" + M + "." + strconv.Itoa(minor+1) + ".0"
			}
		} else {
			ret = ">=" + M + "." + m + "." + p +
				" <" + strconv.Itoa(major+1) + ".0.0"
		}
	}

	return ret
}

func replaceXRanges(re map[string]*regexp.Regexp, s string) string {
	var acc []string
	s = strings.TrimSpace(s)
	parts := regexp.MustCompile("\\s+").Split(s, -1)
	for _, p := range parts {
		acc = append(acc, replaceXRange(re, p))
	}
	return strings.Join(acc, " ")
}

func replaceXRange(re map[string]*regexp.Regexp, s string) string {
	// if we don't match for a hyphen range, return the string unchanged
	if !re["XRANGE"].MatchString(s) {
		return s
	}
	match := re["XRANGE"].FindStringSubmatch(s)

	ret := match[0]
	gtlt := match[1]
	M := match[2]
	m := match[3]
	p := match[4]

	xM := isX(M)
	xm := xM || isX(m)
	xp := xm || isX(p)
	anyX := xp

	if gtlt == "=" && anyX {
		gtlt = ""
	}

	// parsed version numbers as ints
	// the regex ensures they are valid numbers
	major, _ := strconv.Atoi(M)
	minor, _ := strconv.Atoi(m)

	if xM {
		if gtlt == ">" || gtlt == "<" {
			// nothing is allowed
			ret = "<0.0.0"
		} else {
			// nothing is forbidden
			ret = "*"
		}
	} else if len(gtlt) > 0 && anyX {
		// we know patch is an x, because we have any x at all.
		// replace X with 0
		if xm {
			m = "0"
		}
		p = "0"

		if gtlt == ">" {
			// >1 => >=2.0.0
			// >1.2 => >=1.3.0
			// >1.2.3 => >= 1.2.4
			gtlt = ">="
			if xm {
				M = strconv.Itoa(major + 1)
				m = "0"
				p = "0"
			} else {
				m = strconv.Itoa(minor + 1)
				p = "0"
			}
		} else if gtlt == "<=" {
			// <=0.7.x is actually <0.8.0, since any 0.7.x should
			// pass.  Similarly, <=7.x is actually <8.0.0, etc.
			gtlt = "<"
			if xm {
				M = strconv.Itoa(major + 1)
			} else {
				m = strconv.Itoa(minor + 1)
			}
		}

		ret = gtlt + M + "." + m + "." + p
	} else if xm {
		ret = ">=" + M + ".0.0 <" + strconv.Itoa(major+1) + ".0.0"
	} else if xp {
		ret = ">=" + M + "." + m + ".0 <" + M + "." + strconv.Itoa(minor+1) + ".0"
	}

	return ret
}

func replaceStars(re map[string]*regexp.Regexp, s string) string {
	return re["STAR"].ReplaceAllString(strings.TrimSpace(s), ">=0.0.0")
}

func isX(s string) bool {
	return len(s) == 0 || s == "x" || s == "X" || s == "*"
}

// Support * and X as wildcards by rewriting them to "x"
// func replaceStars(s string) string {
// 	s = strings.ReplaceAll(s, "*", "x")
// 	s = strings.ReplaceAll(s, "X", "x")
// 	return s
// }

// splitORParts splits the already cleaned parts by '||'.
// Checks for invalid positions of the operator and returns an
// error if found.
func splitORParts(parts []string) ([][]string, error) {
	var ORparts [][]string
	last := 0
	for i, p := range parts {
		if p == "||" {
			if i == 0 {
				return nil, fmt.Errorf("First element in range is '||'")
			}
			ORparts = append(ORparts, parts[last:i])
			last = i + 1
		}
	}
	if last == len(parts) {
		return nil, fmt.Errorf("Last element in range is '||'")
	}
	ORparts = append(ORparts, parts[last:])
	return ORparts, nil
}

// buildVersionRange takes a slice of 2: operator and version
// and builds a versionRange, otherwise an error.
func buildVersionRange(opStr, vStr string) (*versionRange, error) {
	c := parseComparator(opStr)
	if c == nil {
		return nil, fmt.Errorf("Could not parse comparator %q in %q", opStr, strings.Join([]string{opStr, vStr}, ""))
	}
	v, err := Parse(vStr)
	if err != nil {
		return nil, fmt.Errorf("Could not parse version %q in %q: %s", vStr, strings.Join([]string{opStr, vStr}, ""), err)
	}

	return &versionRange{
		v: v,
		c: c,
	}, nil

}

// inArray checks if a byte is contained in an array of bytes
func inArray(s byte, list []byte) bool {
	for _, el := range list {
		if el == s {
			return true
		}
	}
	return false
}

// splitAndTrim splits a range string by spaces and cleans whitespaces
func splitAndTrim(s string) (result []string) {
	last := 0
	var lastChar byte
	excludeFromSplit := []byte{'>', '<', '='}
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' && !inArray(lastChar, excludeFromSplit) {
			if last < i-1 {
				result = append(result, s[last:i])
			}
			last = i + 1
		} else if s[i] != ' ' {
			lastChar = s[i]
		}
	}
	if last < len(s) {
		result = append(result, s[last:])
	}

	for i, v := range result {
		result[i] = strings.Replace(v, " ", "", -1)
	}

	// parts := strings.Split(s, " ")
	// for _, x := range parts {
	// 	if s := strings.TrimSpace(x); len(s) != 0 {
	// 		result = append(result, s)
	// 	}
	// }
	return
}

// splitComparatorVersion splits the comparator from the version.
// Input must be free of leading or trailing spaces.
func splitComparatorVersion(s string) (string, string, error) {
	// handle case where there is only a wildcard
	if s == "x" {
		return "", "x", nil
	}
	i := strings.IndexFunc(s, unicode.IsDigit)
	if i == -1 {
		return "", "", fmt.Errorf("Could not get version from string: %q", s)
	}
	return strings.TrimSpace(s[0:i]), s[i:], nil
}

// getWildcardType will return the type of wildcard that the
// passed version contains
func getWildcardType(vStr string) wildcardType {
	parts := strings.Split(vStr, ".")
	nparts := len(parts)
	wildcard := parts[nparts-1]

	// handle case where nparts = 1 and no wildcard
	if nparts == 1 {
		_, err := strconv.ParseUint(parts[0], 10, 64)
		if err == nil {
			return minorWildcard
		}
	}

	possibleWildcardType := wildcardTypefromInt(nparts)
	if wildcard == "x" {
		return possibleWildcardType
	}

	return noneWildcard
}

// createVersionFromWildcard will convert a wildcard version
// into a regular version, replacing 'x's with '0's, handling
// special cases like '1.x.x' and '1.x'
func createVersionFromWildcard(vStr string) string {
	// handle 1.x.x
	vStr2 := strings.Replace(vStr, ".x.x", ".x", 1)
	vStr2 = strings.Replace(vStr2, ".x", ".0", 1)
	parts := strings.Split(vStr2, ".")

	// handle 1.x
	if len(parts) == 2 {
		return vStr2 + ".0"
	}

	// handle 1
	if len(parts) == 1 {
		return vStr2 + ".0.0"
	}

	return vStr2
}

// incrementMajorVersion will increment the major version
// of the passed version
func incrementMajorVersion(vStr string) (string, error) {
	parts := strings.Split(vStr, ".")
	i, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", err
	}
	parts[0] = strconv.Itoa(i + 1)
	for i := range parts {
		if i != 0 {
			parts[i] = "0"
		}
	}

	return strings.Join(parts, "."), nil
}

// incrementMajorVersion will increment the minor version
// of the passed version
func incrementMinorVersion(vStr string) (string, error) {
	parts := strings.Split(vStr, ".")
	i, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", err
	}
	parts[1] = strconv.Itoa(i + 1)
	if len(parts) > 2 {
		parts[2] = "0"
	}

	return strings.Join(parts, "."), nil
}

// expandWildcardVersion will expand wildcards inside versions
// following these rules:
//
// * when dealing with patch wildcards:
// >= 1.2.x    will become    >= 1.2.0
// <= 1.2.x    will become    <  1.3.0
// >  1.2.x    will become    >= 1.3.0
// <  1.2.x    will become    <  1.2.0
// != 1.2.x    will become    <  1.2.0 >= 1.3.0
//
// * when dealing with minor wildcards:
// >= 1.x      will become    >= 1.0.0
// <= 1.x      will become    <  2.0.0
// >  1.x      will become    >= 2.0.0
// <  1.0      will become    <  1.0.0
// != 1.x      will become    <  1.0.0 >= 2.0.0
//
// * when dealing with wildcards without
// version operator:
// 1.2.x       will become    >= 1.2.0 < 1.3.0
// 1.x         will become    >= 1.0.0 < 2.0.0
// 1           will become    >= 1.0.0 < 2.0.0
func expandWildcardVersion(parts [][]string) ([][]string, error) {
	var expandedParts [][]string
	for _, p := range parts {
		var newParts []string
		for _, ap := range p {
			if ap == "x" {
				ap = ">=0.0.0"
			} else if major, err := strconv.ParseInt(ap, 10, 64); err == nil {
				// if the part is only a major version number, ex: "1"
				newParts = append(newParts, fmt.Sprintf(">=%d.0.0", major))
				ap = fmt.Sprintf("<%d.0.0", major+1)
			} else if strings.ContainsAny(ap, "x~^><=!") {
				opStr, vStr, err := splitComparatorVersion(ap)
				if err != nil {
					return nil, err
				}

				versionWildcardType := getWildcardType(vStr)
				flatVersion := createVersionFromWildcard(vStr)

				var resultOperator string
				var shouldIncrementVersion bool
				switch opStr {
				case ">":
					switch versionWildcardType {
					case noneWildcard:
						resultOperator = ">"
						shouldIncrementVersion = false
					default:
						resultOperator = ">="
						shouldIncrementVersion = true
					}
				case ">=":
					resultOperator = ">="
				case "<":
					resultOperator = "<"
					shouldIncrementVersion = false
				case "<=":
					resultOperator = "<"
					shouldIncrementVersion = true
					switch versionWildcardType {
					case noneWildcard:
						resultOperator = "<="
						shouldIncrementVersion = false
					default:
						resultOperator = "<"
						shouldIncrementVersion = true
					}
				case "", "=", "==":
					switch versionWildcardType {
					case noneWildcard:
						resultOperator = ""
						shouldIncrementVersion = false
					default:
						newParts = append(newParts, ">="+flatVersion)
						resultOperator = "<"
						shouldIncrementVersion = true
					}
				case "!=", "!":
					switch versionWildcardType {
					case noneWildcard:
						resultOperator = "!"
						shouldIncrementVersion = false
					default:
						newParts = append(newParts, "<"+flatVersion)
						resultOperator = ">="
						shouldIncrementVersion = true
					}
				case "~":
					switch versionWildcardType {
					case noneWildcard, patchWildcard:
						newParts = append(newParts, ">="+flatVersion)
						versionWildcardType = patchWildcard
						resultOperator = "<"
						shouldIncrementVersion = true
					default:
						newParts = append(newParts, ">="+flatVersion)
						resultOperator = "<"
						shouldIncrementVersion = true
					}
				case "^":
					newParts = append(newParts, ">="+flatVersion)
					versionWildcardType = minorWildcard
					resultOperator = "<"
					shouldIncrementVersion = true
				}

				var resultVersion string
				if shouldIncrementVersion {
					switch versionWildcardType {
					case patchWildcard:
						resultVersion, _ = incrementMinorVersion(flatVersion)
					case minorWildcard:
						resultVersion, _ = incrementMajorVersion(flatVersion)
					}
				} else {
					resultVersion = flatVersion
				}

				ap = resultOperator + resultVersion
			}
			newParts = append(newParts, ap)
		}
		expandedParts = append(expandedParts, newParts)
	}

	return expandedParts, nil
}

func parseComparator(s string) comparator {
	switch s {
	case "==":
		fallthrough
	case "":
		fallthrough
	case "=":
		return compEQ
	case ">":
		return compGT
	case ">=":
		return compGE
	case "<":
		return compLT
	case "<=":
		return compLE
	case "!":
		fallthrough
	case "!=":
		return compNE
	}

	return nil
}

// MustParseRange is like ParseRange but panics if the range cannot be parsed.
func MustParseRange(s string) Range {
	r, err := ParseRange(s)
	if err != nil {
		panic(`semver: ParseRange(` + s + `): ` + err.Error())
	}
	return r
}
