package githubapi

import "context"

type diskCacheFill struct {
	done  chan struct{}
	entry *diskCacheEntry
	err   error
}

func (s *diskCacheService) startFill(key string) (*diskCacheFill, int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fills == nil {
		s.fills = make(map[string]*diskCacheFill)
	}
	if fill := s.fills[key]; fill != nil {
		return fill, s.gen, true
	}
	fill := &diskCacheFill{done: make(chan struct{})}
	s.fills[key] = fill
	return fill, s.gen, false
}

func (s *diskCacheService) waitForFill(
	ctx context.Context,
	fill *diskCacheFill,
) (*diskCacheEntry, error) {
	select {
	case <-fill.done:
		return fill.entry, fill.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *diskCacheService) finishFill(
	key string,
	fill *diskCacheFill,
	entry *diskCacheEntry,
	err error,
) {
	s.mu.Lock()
	fill.entry = entry
	fill.err = err
	if err != nil && s.fills[key] == fill {
		delete(s.fills, key)
	}
	close(fill.done)
	s.mu.Unlock()
}

func (s *diskCacheService) cleanupFill(key string, fill *diskCacheFill) {
	s.mu.Lock()
	if s.fills[key] == fill {
		delete(s.fills, key)
	}
	s.mu.Unlock()
}

func (s *diskCacheService) sameGeneration(gen int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gen == gen
}

func (s *diskCacheService) bumpGeneration() {
	s.mu.Lock()
	s.gen++
	s.mu.Unlock()
}
