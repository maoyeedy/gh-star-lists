package githubapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type diskCacheEntry struct {
	Version int          `json:"version"`
	Expiry  time.Time    `json:"expiry"`
	Lists   []StarList   `json:"lists,omitempty"`
	Repos   []Repository `json:"repos,omitempty"`
	Repo    *Repository  `json:"repo,omitempty"`
}

type diskCacheEntryJSON struct {
	Version int                   `json:"version"`
	Expiry  time.Time             `json:"expiry"`
	Lists   []StarList            `json:"lists,omitempty"`
	Repos   []diskCacheRepository `json:"repos,omitempty"`
	Repo    *diskCacheRepository  `json:"repo,omitempty"`
}

type diskCacheRepository struct {
	ID                string   `json:"id,omitempty"`
	NameWithOwner     string   `json:"nameWithOwner"`
	Description       string   `json:"description"`
	IsFork            bool     `json:"isFork"`
	StargazerCount    int      `json:"stargazerCount"`
	PushedAt          string   `json:"pushedAt"`
	URL               string   `json:"url"`
	Language          string   `json:"language"`
	StarredAt         string   `json:"starredAt,omitempty"`
	IsArchived        bool     `json:"isArchived,omitempty"`
	License           string   `json:"license,omitempty"`
	Topics            []string `json:"topics,omitempty"`
	NormNameWithOwner string   `json:"normNameWithOwner,omitempty"`
	NormDescription   string   `json:"normDescription,omitempty"`
	NormLanguage      string   `json:"normLanguage,omitempty"`
}

func (e diskCacheEntry) MarshalJSON() ([]byte, error) {
	entry := diskCacheEntryJSON{
		Version: e.Version,
		Expiry:  e.Expiry,
		Lists:   e.Lists,
		Repos:   repositoriesToDiskCache(e.Repos),
		Repo:    repositoryToDiskCachePtr(e.Repo),
	}
	return json.Marshal(entry)
}

func (e *diskCacheEntry) UnmarshalJSON(data []byte) error {
	var entry diskCacheEntryJSON
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}
	e.Version = entry.Version
	e.Expiry = entry.Expiry
	e.Lists = entry.Lists
	e.Repos = repositoriesFromDiskCache(entry.Repos)
	e.Repo = repositoryFromDiskCachePtr(entry.Repo)
	return nil
}

func repositoriesToDiskCache(repos []Repository) []diskCacheRepository {
	if repos == nil {
		return nil
	}
	out := make([]diskCacheRepository, len(repos))
	for i, repo := range repos {
		out[i] = repositoryToDiskCache(repo)
	}
	return out
}

func repositoryToDiskCachePtr(repo *Repository) *diskCacheRepository {
	if repo == nil {
		return nil
	}
	out := repositoryToDiskCache(*repo)
	return &out
}

func repositoryToDiskCache(repo Repository) diskCacheRepository {
	return diskCacheRepository(repo)
}

func repositoriesFromDiskCache(repos []diskCacheRepository) []Repository {
	if repos == nil {
		return nil
	}
	out := make([]Repository, len(repos))
	for i, repo := range repos {
		out[i] = repositoryFromDiskCache(repo)
	}
	return out
}

func repositoryFromDiskCachePtr(repo *diskCacheRepository) *Repository {
	if repo == nil {
		return nil
	}
	out := repositoryFromDiskCache(*repo)
	return &out
}

func repositoryFromDiskCache(repo diskCacheRepository) Repository {
	return Repository(repo)
}

// NewDiskCacheService wraps inner with an opt-in disk read cache.
// Cache entries use the host, method type, list ID, and withTopics in the
// cache key. Entries expire after TTL (default 5 min). Mutations and
// Invalidate() clear affected disk entries.
func NewDiskCacheService(inner Service, opts DiskCacheOptions) Service {
	ttl := defaultDiskCacheTTL
	if opts.TTL > 0 {
		ttl = opts.TTL
	}
	host := opts.Host
	if host == "" {
		host = "default"
	}
	cacheDir := ""
	if userCacheDir, err := os.UserCacheDir(); err == nil {
		cacheDir = filepath.Join(userCacheDir, diskCacheDirName)
	}
	return &diskCacheService{
		inner:    inner,
		ttl:      ttl,
		host:     host,
		cacheDir: cacheDir,
		fills:    make(map[string]*diskCacheFill),
		maxFiles: diskCacheMaxEntries,
	}
}

func (s *diskCacheService) cachePath(key string) string {
	if s.cacheDir == "" {
		return ""
	}
	h := sha256.Sum256([]byte(key))
	return filepath.Join(s.cacheDir, hex.EncodeToString(h[:16]))
}

func (s *diskCacheService) canonicalKey(method string, parts ...string) string {
	key := fmt.Sprintf("v%d/%s/%s", diskCacheVersion, s.host, method)
	for _, p := range parts {
		key += "/" + p
	}
	return key
}

func (s *diskCacheService) readFromDisk(key string) *diskCacheEntry {
	path := s.cachePath(key)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entry diskCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	if entry.Version != diskCacheVersion {
		return nil
	}
	if time.Now().After(entry.Expiry) {
		return nil
	}
	return &entry
}

func (s *diskCacheService) writeToDisk(
	key string,
	entry *diskCacheEntry,
	fill *diskCacheFill,
	gen int64,
) {
	go func() {
		defer s.cleanupFill(key, fill)
		if s.beforeDiskWrite != nil {
			s.beforeDiskWrite()
		}
		s.writeToDiskSync(key, entry, gen)
	}()
}

func (s *diskCacheService) writeToDiskSync(key string, entry *diskCacheEntry, gen int64) {
	path := s.cachePath(key)
	if path == "" || !s.sameGeneration(gen) {
		return
	}
	entry.Version = diskCacheVersion
	entry.Expiry = time.Now().Add(s.ttl)
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return
	}
	if !s.sameGeneration(gen) {
		_ = os.Remove(tmpPath)
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return
	}
	if !s.sameGeneration(gen) {
		_ = os.Remove(path)
		return
	}
	s.evictOldest()
}

func (s *diskCacheService) evictOldest() {
	if s.cacheDir == "" || s.maxFiles <= 0 {
		return
	}
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return
	}
	type cacheFile struct {
		path    string
		modTime time.Time
	}
	files := make([]cacheFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(s.cacheDir, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		files = append(files, cacheFile{path: path, modTime: info.ModTime()})
	}
	if len(files) <= s.maxFiles {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})
	for _, file := range files[:len(files)-s.maxFiles] {
		_ = os.Remove(file.path)
	}
}
