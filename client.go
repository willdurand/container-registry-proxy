package main

import (
	"context"

	"github.com/google/go-github/v50/github"
)

var packageType = "container"

// GitHubClient describes a (partial) GitHub REST API client.
type GitHubClient interface {
	ListPackages(ctx context.Context, user string, opts *github.PackageListOptions) ([]*github.Package, *github.Response, error)

	PackageGetAllVersions(ctx context.Context, user, packageType, packageName string, opts *github.PackageListOptions) ([]*github.PackageVersion, *github.Response, error)
}
