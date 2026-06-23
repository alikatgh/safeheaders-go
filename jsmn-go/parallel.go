// Package jsmngo provides a fast JSON tokenizer with parallel processing capabilities.
package jsmngo

import (
	"context"
	"runtime"
	"sync"
)

// chunkJob describes a contiguous slice of the input assigned to one worker.
type chunkJob struct {
	id     int
	start  int
	end    int
	offset int
}

// chunkResult carries the tokens produced for a single chunkJob.
type chunkResult struct {
	id   int
	toks []Token
	err  error
}

// parseParallelWithConfig tokenizes JSON in parallel with configuration and context support.
func parseParallelWithConfig(ctx context.Context, json []byte, config *Config) ([]Token, error) {
	splitPoints := findSplitPoints(json)
	numWorkers := runtime.NumCPU()

	// Not enough split points to justify parallelism; fall back to a single pass.
	if len(splitPoints) < numWorkers {
		return parseSerial(json, config)
	}

	jobs := buildChunkJobs(json, splitPoints, numWorkers)
	numJobs := len(jobs)

	jobCh := make(chan chunkJob, numJobs)
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	// Per-chunk token budget: an approximate guard so a single chunk cannot blow
	// past the overall limit. 0 means unlimited.
	maxTokensPerChunk := 0
	if config.MaxTokens > 0 {
		maxTokensPerChunk = config.MaxTokens / numJobs
	}

	resultsCh := make(chan chunkResult, numJobs)
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chunkWorker(ctx, json, jobCh, resultsCh, maxTokensPerChunk, config)
		}()
	}

	wg.Wait()
	close(resultsCh)

	return mergeChunkResults(resultsCh, numJobs, config.MaxTokens)
}

// parseSerial tokenizes the whole input in a single parser pass.
func parseSerial(json []byte, config *Config) ([]Token, error) {
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

// buildChunkJobs groups the top-level values into roughly numChunks contiguous,
// non-overlapping jobs that together cover the entire input. Grouping many
// values per chunk (instead of one chunk per split point) is what makes the
// parallel path pay off: it amortizes the per-chunk parser allocation across
// many values. Each chunk still begins and ends on a top-level boundary, so the
// chunk parses to exactly the tokens it contains (internal separators are
// skipped by the tokenizer just as in a serial pass).
func buildChunkJobs(json []byte, splitPoints []int, numChunks int) []chunkJob {
	values := len(splitPoints) + 1 // number of top-level values
	if numChunks < 1 {
		numChunks = 1
	}
	if numChunks > values {
		numChunks = values
	}

	jobs := make([]chunkJob, 0, numChunks)
	start := 0
	for c := 0; c < numChunks; c++ {
		// This chunk covers value segments [c*values/numChunks, segEnd).
		segEnd := (c + 1) * values / numChunks
		if segEnd >= values {
			jobs = append(jobs, chunkJob{id: c, start: start, end: len(json), offset: start})
			break
		}
		// Cut at the split point that ends the last value in this chunk; the next
		// chunk starts just past that separator.
		boundary := splitPoints[segEnd-1]
		jobs = append(jobs, chunkJob{id: c, start: start, end: boundary, offset: start})
		start = boundary + 1
	}
	return jobs
}

// chunkWorker pulls jobs until the channel drains or the context is canceled.
func chunkWorker(
	ctx context.Context,
	json []byte,
	jobCh <-chan chunkJob,
	resultsCh chan<- chunkResult,
	maxTokensPerChunk int,
	config *Config,
) {
	for {
		select {
		case <-ctx.Done():
			resultsCh <- chunkResult{err: ctx.Err()}
			return
		case j, ok := <-jobCh:
			if !ok {
				return
			}
			toks, err := processChunk(json, j, maxTokensPerChunk, config)
			resultsCh <- chunkResult{id: j.id, toks: toks, err: err}
			if err != nil {
				return
			}
		}
	}
}

// processChunk tokenizes a single chunk and rebases token offsets to be global.
func processChunk(json []byte, j chunkJob, maxTokensPerChunk int, config *Config) ([]Token, error) {
	chunkData := json[j.start:j.end]
	p := NewParser(config.getInitialCapacity(len(chunkData)))

	if _, err := p.Parse(chunkData); err != nil {
		return nil, err
	}

	toks := p.Tokens()
	if maxTokensPerChunk > 0 && len(toks) > maxTokensPerChunk {
		return nil, ErrTooManyTokens
	}

	for i := range toks {
		toks[i].Start += j.offset
		toks[i].End += j.offset
	}
	return toks, nil
}

// mergeChunkResults re-orders worker output by job id and concatenates it,
// failing fast on the first error and enforcing the overall token limit.
func mergeChunkResults(resultsCh <-chan chunkResult, numJobs, maxTokens int) ([]Token, error) {
	jobResults := make([]chunkResult, numJobs)
	for r := range resultsCh {
		if r.err != nil {
			return nil, r.err // Fail fast on first error.
		}
		jobResults[r.id] = r
	}

	totalTokens := 0
	for _, r := range jobResults {
		totalTokens += len(r.toks)
	}
	if maxTokens > 0 && totalTokens > maxTokens {
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
