package main

import (
	"context"
	"os"

	"github.com/maoyeedy/gh-star-lists/internal/command"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func main() {
	ctx := context.Background()
	parsed, err := command.Parse(os.Args[1:])
	if err != nil {
		code := command.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, nil)
		os.Exit(code)
	}
	service := githubapi.NewProductionServiceWithOptions(githubapi.ProductionOptions{
		Host: parsed.Host,
	})
	code := command.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, service)
	os.Exit(code)
}
