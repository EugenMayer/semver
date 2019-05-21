package semver

import (
	"regexp"
	"strconv"
	"strings"
)

// Logic converted from https://github.com/npm/node-semver
//
// This isn't the easiest code to follow, but it is battle-tested
// in the Node ecosystem, so I've tried to keep it as close to the
// original source as I reasonably can

func getRegex() map[string]*regexp.Regexp {
	// Max safe segment length for coercion.
	var MaxSafeComponentLength = 16

	// The actual regexps go on exports.re
	src := make(map[string]string)
	re := make(map[string](*regexp.Regexp))

	// The following Regular Expressions can be used for tokenizing,
	// validating, and parsing SemVer version strings.

	// ## Numeric Identifier
	// A single `0`, or a non-zero digit followed by zero or more digits.

	src["NUMERICIDENTIFIER"] = "0|[1-9]\\d*"
	src["NUMERICIDENTIFIERLOOSE"] = "[0-9]+"

	// ## Non-numeric Identifier
	// Zero or more digits, followed by a letter or hyphen, and then zero or
	// more letters, digits, or hyphens.

	src["NONNUMERICIDENTIFIER"] = "\\d*[a-zA-Z-][a-zA-Z0-9-]*"

	// ## Main Version
	// Three dot-separated numeric identifiers.

	src["MAINVERSION"] = "(" + src["NUMERICIDENTIFIER"] + ")\\." +
		"(" + src["NUMERICIDENTIFIER"] + ")\\." +
		"(" + src["NUMERICIDENTIFIER"] + ")"

	src["MAINVERSIONLOOSE"] = "(" + src["NUMERICIDENTIFIERLOOSE"] + ")\\." +
		"(" + src["NUMERICIDENTIFIERLOOSE"] + ")\\." +
		"(" + src["NUMERICIDENTIFIERLOOSE"] + ")"

	// ## Pre-release Version Identifier
	// A numeric identifier, or a non-numeric identifier.

	src["PRERELEASEIDENTIFIER"] = "(?:" + src["NUMERICIDENTIFIER"] +
		"|" + src["NONNUMERICIDENTIFIER"] + ")"

	src["PRERELEASEIDENTIFIERLOOSE"] = "(?:" + src["NUMERICIDENTIFIERLOOSE"] +
		"|" + src["NONNUMERICIDENTIFIER"] + ")"

	// ## Pre-release Version
	// Hyphen, followed by one or more dot-separated pre-release version
	// identifiers.

	src["PRERELEASE"] = "(?:-(" + src["PRERELEASEIDENTIFIER"] +
		"(?:\\." + src["PRERELEASEIDENTIFIER"] + ")*))"

	src["PRERELEASELOOSE"] = "(?:-?(" + src["PRERELEASEIDENTIFIERLOOSE"] +
		"(?:\\." + src["PRERELEASEIDENTIFIERLOOSE"] + ")*))"

	// ## Build Metadata Identifier
	// Any combination of digits, letters, or hyphens.

	src["BUILDIDENTIFIER"] = "[0-9A-Za-z-]+"

	// ## Build Metadata
	// Plus sign, followed by one or more period-separated build metadata
	// identifiers.

	src["BUILD"] = "(?:\\+(" + src["BUILDIDENTIFIER"] +
		"(?:\\." + src["BUILDIDENTIFIER"] + ")*))"

	// ## Full Version String
	// A main version, followed optionally by a pre-release version and
	// build metadata.

	// Note that the only major, minor, patch, and pre-release sections of
	// the version string are capturing groups.  The build metadata is not a
	// capturing group, because it should not ever be used in version
	// comparison.

	var FULLPLAIN = "v?" + src["MAINVERSION"] +
		src["PRERELEASE"] + "?" +
		src["BUILD"] + "?"

	src["FULL"] = "^" + FULLPLAIN + "$"

	// like full, but allows v1.2.3 and =1.2.3, which people do sometimes.
	// also, 1.0.0alpha1 (prerelease without the hyphen) which is pretty
	// common in the npm registry.
	var LOOSEPLAIN = "[v=\\s]*" + src["MAINVERSIONLOOSE"] +
		src["PRERELEASELOOSE"] + "?" +
		src["BUILD"] + "?"

	src["LOOSE"] = "^" + LOOSEPLAIN + "$"

	src["GTLT"] = "((?:<|>)?=?)"

	// Something like "2.*" or "1.2.x".
	// Note that "x.x" is a valid xRange identifer, meaning "any version"
	// Only the first item is strictly required.
	src["XRANGEIDENTIFIERLOOSE"] = src["NUMERICIDENTIFIERLOOSE"] + "|x|X|\\*"
	src["XRANGEIDENTIFIER"] = src["NUMERICIDENTIFIER"] + "|x|X|\\*"

	src["XRANGEPLAIN"] = "[v=\\s]*(" + src["XRANGEIDENTIFIER"] + ")" +
		"(?:\\.(" + src["XRANGEIDENTIFIER"] + ")" +
		"(?:\\.(" + src["XRANGEIDENTIFIER"] + ")" +
		"(?:" + src["PRERELEASE"] + ")?" +
		src["BUILD"] + "?" +
		")?)?"

	src["XRANGEPLAINLOOSE"] = "[v=\\s]*(" + src["XRANGEIDENTIFIERLOOSE"] + ")" +
		"(?:\\.(" + src["XRANGEIDENTIFIERLOOSE"] + ")" +
		"(?:\\.(" + src["XRANGEIDENTIFIERLOOSE"] + ")" +
		"(?:" + src["PRERELEASELOOSE"] + ")?" +
		src["BUILD"] + "?" +
		")?)?"

	src["XRANGE"] = "^" + src["GTLT"] + "\\s*" + src["XRANGEPLAIN"] + "$"
	src["XRANGELOOSE"] = "^" + src["GTLT"] + "\\s*" + src["XRANGEPLAINLOOSE"] + "$"

	// Coercion.
	// Extract anything that could conceivably be a part of a valid semver
	src["COERCE"] = "(?:^|[^\\d])" +
		"(\\d{1," + strconv.Itoa(MaxSafeComponentLength) + "})" +
		"(?:\\.(\\d{1," + strconv.Itoa(MaxSafeComponentLength) + "}))?" +
		"(?:\\.(\\d{1," + strconv.Itoa(MaxSafeComponentLength) + "}))?" +
		"(?:$|[^\\d])"

	// Tilde ranges.
	// Meaning is "reasonably at or greater than"
	src["LONETILDE"] = "(?:~>?)"

	src["TILDETRIM"] = "(\\s*)" + src["LONETILDE"] + "\\s+"

	src["TILDE"] = "^" + src["LONETILDE"] + src["XRANGEPLAIN"] + "$"
	src["TILDELOOSE"] = "^" + src["LONETILDE"] + src["XRANGEPLAINLOOSE"] + "$"

	// Caret ranges.
	// Meaning is "at least and backwards compatible with"
	src["LONECARET"] = "(?:\\^)"

	src["CARETTRIM"] = "(\\s*)" + src["LONECARET"] + "\\s+"

	src["CARET"] = "^" + src["LONECARET"] + src["XRANGEPLAIN"] + "$"
	src["CARETLOOSE"] = "^" + src["LONECARET"] + src["XRANGEPLAINLOOSE"] + "$"

	// A simple gt/lt/eq thing, or just "" to indicate "any version"
	src["COMPARATORLOOSE"] = "^" + src["GTLT"] + "\\s*(" + LOOSEPLAIN + ")$|^$"
	src["COMPARATOR"] = "^" + src["GTLT"] + "\\s*(" + FULLPLAIN + ")$|^$"

	// An expression to strip any whitespace between the gtlt and the thing
	// it modifies, so that `> 1.2.3` ==> `>1.2.3`
	src["COMPARATORTRIM"] = "(\\s*)" + src["GTLT"] +
		"\\s*(" + LOOSEPLAIN + "|" + src["XRANGEPLAIN"] + ")"

	// Something like `1.2.3 - 1.2.4`
	// Note that these all use the loose form, because they"ll be
	// checked against either the strict or loose comparator form
	// later.
	src["HYPHENRANGE"] = "^\\s*(" + src["XRANGEPLAIN"] + ")" +
		"\\s+-\\s+" +
		"(" + src["XRANGEPLAIN"] + ")" +
		"\\s*$"

	src["HYPHENRANGELOOSE"] = "^\\s*(" + src["XRANGEPLAINLOOSE"] + ")" +
		"\\s+-\\s+" +
		"(" + src["XRANGEPLAINLOOSE"] + ")" +
		"\\s*$"

	// Star ranges basically just allow anything at all.
	src["STAR"] = "(<|>)?=?\\s*\\*"

	for key, exp := range src {
		re[key] = regexp.MustCompile(exp)
	}

	return re
}

func parseRange(s string) []string {
	var out []string
	s = strings.TrimSpace(s)
	re := getRegex()

	// `1.2.3 - 1.2.4` => `>=1.2.3 <=1.2.4`
	s = hyphenReplace(re, s)

	// `> 1.2.3 < 1.2.5` => `>1.2.3 <1.2.5`
	s = re["COMPARATORTRIM"].ReplaceAllString(s, "$1$2$3")
	// `~ 1.2.3` => `~1.2.3`
	s = re["TILDETRIM"].ReplaceAllString(s, "$1~")
	// `^ 1.2.3` => `^1.2.3
	s = re["CARETTRIM"].ReplaceAllString(s, "$1^")
	// normalize spaces
	s = strings.Join(regexp.MustCompile("\\s+").Split(s, -1), " ")

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
	s = replaceV(re, s)
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

func replaceV(re map[string]*regexp.Regexp, s string) string {
	return s
}

func isX(s string) bool {
	return len(s) == 0 || s == "x" || s == "X" || s == "*"
}
