package main

import (
	"context"
	"os"

	"github.com/maoyeedy/gh-star-lists/internal/command"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func main() {
	ctx := context.Background()
	service := githubapi.NewProductionService()
	code := command.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, service)
	os.Exit(code)
}
