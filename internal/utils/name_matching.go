package utils

import (
	"regexp"
	"strings"
	"unicode"
)

// PersianCharMap maps various forms of Persian/Arabic characters to a normalized form
var persianCharMap = map[rune]rune{
	'ك': 'ک', // Arabic kaf to Persian kaf
	'ي': 'ی', // Arabic ya to Persian ya
	'ة': 'ه', // Arabic ta marbuta to Persian he
	'ؤ': 'و', // Arabic waw with hamza to waw
	'إ': 'ا', // Arabic alef with hamza below to alef
	'أ': 'ا', // Arabic alef with hamza above to alef
	'آ': 'ا', // Arabic alef with madda to alef
	'ٱ': 'ا', // Arabic alef wasla to alef
	'ئ': 'ی', // Arabic ya with hamza to ya
	'ى': 'ی', // Arabic alef maksura to ya
}

// CommonPersianNameVariants maps common name variations
var commonPersianNameVariants = map[string][]string{
	"محمد":    {"محمد", "محمّد", "محمدی"},
	"علی":     {"علی", "علي"},
	"حسن":     {"حسن", "حسین"},
	"رضا":     {"رضا", "رضی"},
	"قربان":   {"قربان", "غربان"},
	"حسین":    {"حسین", "حسين"},
	"اکبر":    {"اکبر", "اكبر"},
	"اصغر":    {"اصغر", "اصفر"},
	"ابراهیم": {"ابراهیم", "ابراهيم", "براهیم"},
}

// NormalizePersianName normalizes a Persian name for comparison
// It handles:
// - Character normalization (Arabic to Persian variants)
// - Space removal between name parts (محمد علی -> محمدعلی)
// - Diacritic removal
// - Case normalization for mixed scripts
func NormalizePersianName(name string) string {
	// Convert to lowercase for any Latin characters
	name = strings.ToLower(name)

	// Normalize Persian/Arabic characters
	var normalized strings.Builder
	for _, r := range name {
		// Check if there's a mapping for this character
		if mapped, ok := persianCharMap[r]; ok {
			normalized.WriteRune(mapped)
		} else if unicode.Is(unicode.Mn, r) {
			// Skip diacritical marks (تشدید، فتحه، کسره، ضمه، etc.)
			continue
		} else if !unicode.IsSpace(r) {
			// Keep non-space characters
			normalized.WriteRune(r)
		}
		// Skip spaces to normalize "محمد علی" to "محمدعلی"
	}

	return normalized.String()
}

// NormalizePersianNameKeepSpaces normalizes but keeps spaces (for display)
func NormalizePersianNameKeepSpaces(name string) string {
	name = strings.ToLower(name)
	var normalized strings.Builder
	for _, r := range name {
		if mapped, ok := persianCharMap[r]; ok {
			normalized.WriteRune(mapped)
		} else if unicode.Is(unicode.Mn, r) {
			continue
		} else {
			normalized.WriteRune(r)
		}
	}
	return strings.TrimSpace(normalized.String())
}

// LevenshteinDistance calculates the edit distance between two strings
func LevenshteinDistance(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)

	len1 := len(r1)
	len2 := len(r2)

	if len1 == 0 {
		return len2
	}
	if len2 == 0 {
		return len1
	}

	// Create matrix
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	// Initialize first column
	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}

	// Initialize first row
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	// Fill in the rest of the matrix
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len1][len2]
}

// CalculateNameSimilarity returns a similarity score between 0 and 1
// 1 means identical, 0 means completely different
func CalculateNameSimilarity(name1, name2 string) float64 {
	// Normalize both names
	norm1 := NormalizePersianName(name1)
	norm2 := NormalizePersianName(name2)

	// Exact match after normalization
	if norm1 == norm2 {
		return 1.0
	}

	// Calculate Levenshtein distance
	distance := LevenshteinDistance(norm1, norm2)
	maxLen := max(len([]rune(norm1)), len([]rune(norm2)))

	if maxLen == 0 {
		return 1.0
	}

	// Convert distance to similarity score
	similarity := 1.0 - float64(distance)/float64(maxLen)

	return similarity
}

// NameMatchResult represents a potential duplicate match
type NameMatchResult struct {
	PersonID   string  `json:"person_id"`
	Name       string  `json:"name"`
	Similarity float64 `json:"similarity"`
	MatchType  string  `json:"match_type"` // "exact", "normalized", "similar", "ai"
}

// FindSimilarNames finds names in the list that are similar to the given name
// Returns matches with similarity >= threshold
func FindSimilarNames(targetName string, existingNames map[string]string, threshold float64) []NameMatchResult {
	var results []NameMatchResult

	normalizedTarget := NormalizePersianName(targetName)

	for personID, existingName := range existingNames {
		// Exact match
		if strings.EqualFold(targetName, existingName) {
			results = append(results, NameMatchResult{
				PersonID:   personID,
				Name:       existingName,
				Similarity: 1.0,
				MatchType:  "exact",
			})
			continue
		}

		// Normalized exact match (handles محمد علی vs محمدعلی)
		normalizedExisting := NormalizePersianName(existingName)
		if normalizedTarget == normalizedExisting {
			results = append(results, NameMatchResult{
				PersonID:   personID,
				Name:       existingName,
				Similarity: 0.99,
				MatchType:  "normalized",
			})
			continue
		}

		// Fuzzy match using Levenshtein distance
		similarity := CalculateNameSimilarity(targetName, existingName)
		if similarity >= threshold {
			results = append(results, NameMatchResult{
				PersonID:   personID,
				Name:       existingName,
				Similarity: similarity,
				MatchType:  "similar",
			})
		}
	}

	// Sort by similarity (highest first)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// ContainsPersianCharacters checks if a string contains Persian/Arabic characters
func ContainsPersianCharacters(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Arabic, r) {
			return true
		}
	}
	return false
}

// ExtractNameParts splits a name into parts, handling both spaces and Persian zero-width joiners
func ExtractNameParts(name string) []string {
	// Replace zero-width non-joiner with space
	name = strings.ReplaceAll(name, "\u200c", " ")
	// Split by whitespace
	re := regexp.MustCompile(`\s+`)
	parts := re.Split(name, -1)
	// Filter empty parts
	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
