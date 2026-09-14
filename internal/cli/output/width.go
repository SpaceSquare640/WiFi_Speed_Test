package output

import "strings"

// displayWidth returns how many terminal columns a string occupies.
//
// It is not the rune count. A CJK ideograph occupies two columns, so padding by
// rune count leaves every Chinese label misaligned against its English
// equivalent — visible only when the interface is actually run in Chinese.
func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runeWidth(r)
	}
	return width
}

// runeWidth reports the columns one rune occupies.
//
// The ranges cover what this interface can actually emit: CJK ideographs, the
// kana and Hangul blocks, and full-width punctuation. Combining marks and the
// rarer double-width blocks are out of scope; being wrong about a character the
// tool never prints costs nothing.
func runeWidth(r rune) int {
	switch {
	case r < 0x1100:
		return 1
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0x303E, // CJK radicals, punctuation
		r >= 0x3041 && r <= 0x33FF, // kana, CJK compatibility
		r >= 0x3400 && r <= 0x4DBF, // CJK extension A
		r >= 0x4E00 && r <= 0x9FFF, // CJK unified ideographs
		r >= 0xA000 && r <= 0xA4CF, // Yi
		r >= 0xAC00 && r <= 0xD7A3, // Hangul syllables
		r >= 0xF900 && r <= 0xFAFF, // CJK compatibility ideographs
		r >= 0xFE30 && r <= 0xFE6F, // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFF60, // full-width forms
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x20000 && r <= 0x3FFFD: // CJK extension B and beyond
		return 2
	default:
		return 1
	}
}

// padRight pads a string to the given display width.
//
// It must be applied before any colour is added: an escape sequence occupies no
// columns but plenty of bytes, and padding a painted string aligns the invisible
// characters instead of the visible ones.
func padRight(s string, width int) string {
	gap := width - displayWidth(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}
