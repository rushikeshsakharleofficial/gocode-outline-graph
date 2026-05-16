// Package search provides symbol-search capabilities over the indexed database.
package search

import (
	"sort"

	"gocode-outline-graph/internal/db"
)

// rrfK is the constant used in Reciprocal Rank Fusion scoring.
const rrfK = 60

// rrfEntry accumulates combined RRF scores for a single symbol.
type rrfEntry struct {
	symbol db.Symbol
	score  float64
}

// Searcher wraps a Database and exposes symbol-search methods.
type Searcher struct {
	database *db.Database
}

// New creates a Searcher backed by the given database.
func New(database *db.Database) *Searcher {
	return &Searcher{database: database}
}

// FTSSearch delegates directly to db.FTSSearch.
func (s *Searcher) FTSSearch(query string, limit int) ([]db.Symbol, error) {
	return s.database.FTSSearch(query, limit)
}

// KeywordSearch delegates directly to db.KeywordSearch.
func (s *Searcher) KeywordSearch(query string, limit int) ([]db.Symbol, error) {
	return s.database.KeywordSearch(query, limit)
}

// ResolveEditTarget finds the best symbol match for editing by merging FTS and
// keyword search results using Reciprocal Rank Fusion (RRF).
//
// Algorithm:
//  1. Run FTSSearch(query, limit).
//  2. If fewer than limit results, also run KeywordSearch(query, limit).
//  3. Score each occurrence by 1 / (rrfK + rank).
//  4. Deduplicate by symbol ID, summing scores across both result lists.
//  5. Return the top limit symbols sorted by descending RRF score.
func (s *Searcher) ResolveEditTarget(query string, limit int) ([]db.Symbol, error) {
	ftsResults, err := s.database.FTSSearch(query, limit)
	if err != nil {
		return nil, err
	}

	var kwResults []db.Symbol
	if len(ftsResults) < limit {
		kwResults, err = s.database.KeywordSearch(query, limit)
		if err != nil {
			return nil, err
		}
	}

	// Accumulate RRF scores keyed by symbol ID.
	scores := make(map[int64]*rrfEntry)

	addResults := func(results []db.Symbol) {
		for rank, sym := range results {
			contribution := 1.0 / float64(rrfK+rank+1)
			if entry, exists := scores[sym.ID]; exists {
				entry.score += contribution
			} else {
				symCopy := sym
				scores[sym.ID] = &rrfEntry{symbol: symCopy, score: contribution}
			}
		}
	}

	addResults(ftsResults)
	addResults(kwResults)

	// Collect and sort by descending RRF score.
	merged := make([]*rrfEntry, 0, len(scores))
	for _, entry := range scores {
		merged = append(merged, entry)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].score != merged[j].score {
			return merged[i].score > merged[j].score
		}
		// Stable tie-break: lower ID first (deterministic).
		return merged[i].symbol.ID < merged[j].symbol.ID
	})

	// Truncate to limit.
	if len(merged) > limit {
		merged = merged[:limit]
	}

	result := make([]db.Symbol, len(merged))
	for i, entry := range merged {
		result[i] = entry.symbol
	}
	return result, nil
}
