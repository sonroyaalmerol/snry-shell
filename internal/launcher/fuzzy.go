package launcher

import (
	"sort"
	"strings"
)

// FuzzyScore returns a relevance score for query against target.
// Higher is more relevant. 0 means no match.
//
// Scoring:
//   3 — target starts with query (prefix match)
//   2 — target contains query (substring match)
//   1 — all query characters appear in order in target (subsequence match)
//   0 — no match
func FuzzyScore(query, target string) int {
	if query == "" {
		return 1
	}
	q := strings.ToLower(query)
	t := strings.ToLower(target)

	if strings.HasPrefix(t, q) {
		return 3
	}
	if strings.Contains(t, q) {
		return 2
	}
	// Subsequence check.
	qi := 0
	for _, c := range t {
		if qi < len(q) && rune(q[qi]) == c {
			qi++
		}
	}
	if qi == len(q) {
		return 1
	}
	return 0
}

// Filter returns apps matching query, sorted by descending relevance.
func Filter(apps []App, query string) []App {
	type scored struct {
		app   App
		score int
	}
	var results []scored
	for _, a := range apps {
		s := max(FuzzyScore(query, a.Name), FuzzyScore(query, a.Comment))
		if s > 0 {
			results = append(results, scored{a, s})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	out := make([]App, len(results))
	for i, r := range results {
		out[i] = r.app
	}
	return out
}
