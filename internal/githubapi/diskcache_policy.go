package githubapi

import "time"

const (
	diskCacheVersion    = 2
	defaultDiskCacheTTL = 5 * time.Minute
	diskCacheDirName    = "gh-star-lists"
	diskCacheMaxEntries = 200
)

// DiskCacheOptions configures the disk-backed cache service.
type DiskCacheOptions struct {
	TTL  time.Duration
	Host string
}
