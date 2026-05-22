package command

import (
	"context"
	"io"
	"time"

	"github.com/cli/go-gh/v2/pkg/browser"

	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"github.com/maoyeedy/gh-star-lists/internal/tui"
)

var runTUI = tui.Run

func RunTUIForTest(
	fn func(context.Context, githubapi.Service, tui.Options) error,
) func(context.Context, githubapi.Service, tui.Options) error {
	prev := runTUI
	runTUI = fn
	return prev
}

type combinedInvalidator struct {
	githubapi.Service
	invalidateDisk func()
}

func (c *combinedInvalidator) Invalidate() {
	if inv, ok := c.Service.(interface{ Invalidate() }); ok {
		inv.Invalidate()
	}
	if c.invalidateDisk != nil {
		c.invalidateDisk()
	}
}

func newCombinedInvalidator(
	svc githubapi.Service,
	diskSvc githubapi.Service,
) githubapi.Service {
	return &combinedInvalidator{
		Service: svc,
		invalidateDisk: func() {
			if inv, ok := diskSvc.(interface{ Invalidate() }); ok {
				inv.Invalidate()
			}
		},
	}
}

func launchTUI(
	ctx context.Context,
	stderr io.Writer,
	parsed Parsed,
	svc, originalSvc githubapi.Service,
	cacheTTL time.Duration,
	diagnosticOpts format.Options,
) int {
	tuiSvc := wrapServiceForTUI(svc, originalSvc, cacheTTL, parsed.Host)
	if err := runTUI(ctx, tuiSvc, tui.Options{
		NoColor: parsed.NoColor,
		Mouse:   parsed.Mouse,
		Stderr:  stderr,
		OpenBrowser: func(url string) error {
			_ = browser.New("", io.Discard, io.Discard).Browse(url)
			return nil
		},
	}); err != nil {
		_ = writeErrorDiagnostic(stderr, diagnosticOpts, "%v\n", err)
		return ExitFailure
	}
	return ExitSuccess
}

func wrapServiceForTUI(
	svc githubapi.Service,
	originalSvc githubapi.Service,
	cacheTTL time.Duration,
	host string,
) githubapi.Service {
	if cacheTTL <= 0 {
		return svc
	}
	hostVal := host
	if hostVal == "" {
		hostVal = "default"
	}
	diskSvc := githubapi.NewDiskCacheService(originalSvc, githubapi.DiskCacheOptions{
		TTL:  cacheTTL,
		Host: hostVal,
	})
	memSvc := githubapi.NewCacheServiceWithOptions(diskSvc, githubapi.CacheOptions{TTL: cacheTTL})
	return newCombinedInvalidator(memSvc, diskSvc)
}
