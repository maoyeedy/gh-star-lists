package githubapi

import (
	"os"
	"path/filepath"
)

// Invalidate removes all disk cache entries. Used by the TUI for manual refresh.
func (s *diskCacheService) Invalidate() {
	s.bumpGeneration()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheDir == "" {
		return
	}
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			_ = os.Remove(filepath.Join(s.cacheDir, entry.Name()))
		}
	}
}

func (s *diskCacheService) invalidateLists() {
	s.removeFile(s.canonicalKey("lists"))
}

func (s *diskCacheService) invalidateRepos(listID string) {
	s.removeFile(s.canonicalKey("repos", listID, "topics:true"))
	s.removeFile(s.canonicalKey("repos", listID, "topics:false"))
}

func (s *diskCacheService) invalidateStarred() {
	s.removeFile(s.canonicalKey("starred", "topics:true"))
	s.removeFile(s.canonicalKey("starred", "topics:false"))
}

func (s *diskCacheService) invalidateAll() {
	s.Invalidate()
}

func (s *diskCacheService) removeFile(key string) {
	s.bumpGeneration()
	path := s.cachePath(key)
	if path == "" {
		return
	}
	_ = os.Remove(path)
}
