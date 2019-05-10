package semver

import (
	"regexp"
	"strconv"
)

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
	// re["TILDETRIM"] = new RegExp(src["TILDETRIM"], "g")
	// re["TILDETRIM"] = regexp.MustCompile("(?g)" + src["TILDETRIM"])
	// var tildeTrimReplace = "$1~"

	src["TILDE"] = "^" + src["LONETILDE"] + src["XRANGEPLAIN"] + "$"
	src["TILDELOOSE"] = "^" + src["LONETILDE"] + src["XRANGEPLAINLOOSE"] + "$"

	// Caret ranges.
	// Meaning is "at least and backwards compatible with"
	src["LONECARET"] = "(?:\\^)"

	src["CARETTRIM"] = "(\\s*)" + src["LONECARET"] + "\\s+"
	// re["CARETTRIM"] = new RegExp(src["CARETTRIM"], "g")
	// re["CARETTRIM"] = regexp.MustCompile("(?g)" + src["CARETTRIM"])
	// var caretTrimReplace = "$1^"

	src["CARET"] = "^" + src["LONECARET"] + src["XRANGEPLAIN"] + "$"
	src["CARETLOOSE"] = "^" + src["LONECARET"] + src["XRANGEPLAINLOOSE"] + "$"

	// A simple gt/lt/eq thing, or just "" to indicate "any version"
	src["COMPARATORLOOSE"] = "^" + src["GTLT"] + "\\s*(" + LOOSEPLAIN + ")$|^$"
	src["COMPARATOR"] = "^" + src["GTLT"] + "\\s*(" + FULLPLAIN + ")$|^$"

	// An expression to strip any whitespace between the gtlt and the thing
	// it modifies, so that `> 1.2.3` ==> `>1.2.3`
	src["COMPARATORTRIM"] = "(\\s*)" + src["GTLT"] +
		"\\s*(" + LOOSEPLAIN + "|" + src["XRANGEPLAIN"] + ")"

	// this one has to use the /g flag
	// re["COMPARATORTRIM"] = new RegExp(src["COMPARATORTRIM"], "g")
	// re["COMPARATORTRIM"] = regexp.MustCompile("(?g)" + src["COMPARATORTRIM"])
	// var comparatorTrimReplace = "$1$2$3"

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
		if _, ok := re[key]; !ok {
			re[key] = regexp.MustCompile(exp)
		}
	}

	return re
}
