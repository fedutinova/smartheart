package repository

import "strings"

// antonymPairs lists ECG-domain term pairs where one word appearing in the
// incoming question and the other in the cached question signals a clinically
// different topic. A cache hit is vetoed when the questions differ on any axis.
//
// Each inner slice is an axis: the guard checks that the two questions do not
// fall on opposite sides. Terms are lowercase Russian; matching is substring.
// Pairs use word stems to match across Russian grammatical forms.
var antonymPairs = [][2]string{
	{"левого", "правого"},         // левый/правый желудочек, предсердие
	{"левой", "правой"},           // блокада левой/правой ножки (genitive/instrumental)
	{"лнпг", "пнпг"},              // abbreviations for bundle branches
	{"удлинени", "укорочени"},     // удлинение/укорочение QT (stem covers all cases)
	{"удлинённ", "укороченн"},     // adjectival forms: удлинённый/укороченный
	{"с подъёмом", "без подъёма"}, // ST elevation
	{"stemi", "nstemi"},
	{"гиперкалиеми", "гипокалиеми"}, // stem covers гиперкалиемия/гиперкалиемии/etc.
	{"гипер", "гипо"},               // broad guard: гипертрофия/гипотония, etc.
	{"тахикарди", "брадикарди"},      // stem covers тахикардия/тахикардии/тахикардию
	{"фибрилляци", "трепетани"},      // AF vs flutter (stem covers all cases)
	{"высок", "отрицательн"},         // peaked vs negative T waves
	{"инверси", "высок"},             // T wave inversion vs peaked T
}

// HasContradiction returns true when incoming and cached questions differ on a
// known antonym axis, meaning the cached answer is clinically unsafe to reuse.
// Both strings must already be normalised (lowercase, trimmed).
func HasContradiction(incoming, cached string) bool {
	for _, pair := range antonymPairs {
		a, b := pair[0], pair[1]
		inA := strings.Contains(incoming, a)
		inB := strings.Contains(incoming, b)
		cachedA := strings.Contains(cached, a)
		cachedB := strings.Contains(cached, b)

		// Contradiction: one question has term A, the other has term B (not both).
		if (inA && !inB) && (cachedB && !cachedA) {
			return true
		}
		if (inB && !inA) && (cachedA && !cachedB) {
			return true
		}
	}
	return false
}
