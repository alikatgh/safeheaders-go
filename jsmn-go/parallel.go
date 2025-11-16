// Package jsmngo provides a fast JSON tokenizer with parallel processing capabilities.
package jsmngo

import (
	"context"
	"runtime"
	"sync"
)

// parseParallelWithConfig tokenizes JSON in parallel with configuration and context support.
func parseParallelWithConfig(ctx context.Context, json []byte, config *Config) ([]Token, error) {
	splitPoints := findSplitPoints(json)
	numWorkers := runtime.NumCPU()

	// Not enough split points to justify parallelism.
	if len(splitPoints) < numWorkers {
		capacity := config.getInitialCapacity(len(json))
		p := NewParser(capacity)
		count, err := p.Parse(json)
		if err != nil {
			return nil, err
		}

		if config.MaxTokens > 0 && count > config.MaxTokens {
			return nil, ErrTooManyTokens
		}

		return p.Tokens(), nil
	}

	// Define chunks for workers
	type job struct {
		id     int
		start  int
		end    int
		offset int
	}

	numJobs := len(splitPoints) + 1
	jobs := make(chan job, numJobs)
	lastSplit := 0
	for i, split := range splitPoints {
		jobs <- job{id: i, start: lastSplit, end: split, offset: lastSplit}
		lastSplit = split + 1
	}
	jobs <- job{id: len(splitPoints), start: lastSplit, end: len(json), offset: lastSplit}
	close(jobs)

	// Each worker returns its result to be merged later
	type result struct {
		id   int
		toks []Token
		err  error
	}

	resultsCh := make(chan result, numJobs)
	var wg sync.WaitGroup

	// Context-aware workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					// Context canceled, send error and exit
					resultsCh <- result{err: ctx.Err()}
					return
				case j, ok := <-jobs:
					if !ok {
						return
					}

					chunkData := json[j.start:j.end]
					capacity := config.getInitialCapacity(len(chunkData))
					p := NewParser(capacity)

					_, err := p.Parse(chunkData)
					if err != nil {
						resultsCh <- result{id: j.id, err: err}
						return
					}

					toks := p.Tokens()

					// Check token limit for this chunk
					if config.MaxTokens > 0 && len(toks) > config.MaxTokens/numJobs {
						// Approximate check per chunk
						resultsCh <- result{id: j.id, err: ErrTooManyTokens}
						return
					}

					// Fix offsets to be global
					for i := range toks {
						toks[i].Start += j.offset
						toks[i].End += j.offset
					}

					resultsCh <- result{id: j.id, toks: toks}
				}
			}
		}()
	}

	wg.Wait()
	close(resultsCh)

	// Collect and re-order results
	jobResults := make([]result, numJobs)
	for r := range resultsCh {
		if r.err != nil {
			return nil, r.err // Fail fast on first error
		}
		jobResults[r.id] = r
	}

	// Merge results in the correct order
	var totalTokens int
	for _, r := range jobResults {
		totalTokens += len(r.toks)
	}

	// Final token count check
	if config.MaxTokens > 0 && totalTokens > config.MaxTokens {
		return nil, ErrTooManyTokens
	}

	finalTokens := make([]Token, 0, totalTokens)
	for _, r := range jobResults {
		finalTokens = append(finalTokens, r.toks...)
	}

	return finalTokens, nil
}

// ParseParallelWithContext is a wrapper that uses default config with context support.
// This maintains backward compatibility while adding context support.
func ParseParallelWithContext(ctx context.Context, json []byte) ([]Token, error) {
	return ParseWithConfig(ctx, json, DefaultConfig())
}
