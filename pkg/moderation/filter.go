package moderation

import (
	"regexp"
	"strings"
	"unicode"
)

// HatePatterns captures neo-Nazi slogans, numeric codes, terrorist organizations, and extremist symbols.
var HatePatterns = []*regexp.Regexp{
	// Numeric codes and slogans with spacing/punctuation variations
	regexp.MustCompile(`(?i)\b(14[\s\-_/]*88|14\s*words|fourteen\s*words)\b`),
	regexp.MustCompile(`(?i)\b(b[1l][o0]{2}d\s*(?:and|&)\s*s[o0][i1l]{2}|blut\s*und\s*boden)\b`),
	regexp.MustCompile(`(?i)\b(sieg\s*heil|heil\s*hitler|white\s*power|aryan\s*brotherhood|ku\s*klux\s*klan|kkk)\b`),
	// Neo-Nazi terrorist groups and paramilitary factions
	regexp.MustCompile(`(?i)\b(atomwaffen|combat\s*18|blood\s*&\s*honour|blood\s*and\s*honour|nordic\s*resistance|order\s*of\s*nine\s*angles|o9a|national\s*socialist\s*order)\b`),
	// Slurs and dehumanizing hate speech
	regexp.MustCompile(`(?i)\b(nigger[s]?|nigga[s]?|kike[s]?|gook[s]?|chink[s]?|wetback[s]?|faggot[s]?)\b`),
}

// BannedRunes captures Unicode swastikas and Nazi runic symbols.
var BannedRunes = map[rune]bool{
	'\u534D': true, // Left-facing swastika / Wan
	'\u5350': true, // Right-facing swastika / Manji
	'\u16CB': true, // Sig / Sowilo runic symbol (SS)
	'\u16DF': true, // Othala rune variant when used in neo-Nazi heraldry
}

// NormalizeText normalizes leetspeak, stripped punctuation, and excessive spacing.
func NormalizeText(input string) string {
	// Strip zero-width and invisible characters
	clean := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Variation_Selector, r) || r == '\u200B' || r == '\u200C' || r == '\u200D' || r == '\uFEFF' {
			return -1
		}
		return r
	}, input)

	// Replace common leetspeak substitutions
	replacer := strings.NewReplacer(
		"1", "i",
		"0", "o",
		"3", "e",
		"4", "a",
		"@", "a",
		"$", "s",
		"5", "s",
		"7", "t",
	)
	normalized := replacer.Replace(clean)

	return strings.ToLower(normalized)
}

// ScanContent evaluates text for zero-tolerance hate content, symbols, and slurs.
func ScanContent(content string) (allowed bool, violation string) {
	// 1. Check for banned Unicode runes/symbols
	for _, r := range content {
		if BannedRunes[r] {
			return false, "Post contains prohibited neo-Nazi runic or swastika symbols"
		}
	}

	// 2. Direct pattern check
	for _, pattern := range HatePatterns {
		if pattern.MatchString(content) {
			return false, "Post contains prohibited neo-Nazi slogans, hate group affiliations, or slurs"
		}
	}

	// 3. Leetspeak normalized pattern check
	normalized := NormalizeText(content)
	for _, pattern := range HatePatterns {
		if pattern.MatchString(normalized) {
			return false, "Post contains obfuscated neo-Nazi terminology or hate speech"
		}
	}

	return true, ""
}
