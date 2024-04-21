package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/go-github/v50/github"
)

const (
	defaultHost        = "127.0.0.1"
	defaultPort        = "10000"
	defaultUpstreamURL = "https://ghcr.io"
)

type containerProxy struct {
	ghClient GitHubClient
	owner    string
}

// NewProxy returns an instance of container proxy, which implements the Docker
// Registry HTTP API V2.
func NewProxy(addr string, ghClient GitHubClient, rawUpstreamURL, owner string) *http.Server {
	proxy := containerProxy{
		ghClient: ghClient,
		owner:    owner,
	}

	if owner != "" {
		log.Printf("packages owner set to: %s", owner)
	}

	// Create an upstream (reverse) proxy to handle the requests not supported by
	// the container proxy.
	upstreamURL, err := url.Parse(rawUpstreamURL)
	if err != nil {
		log.Fatal(err)
	}
	upstreamProxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstreamURL)
		},
	}

	router := chi.NewRouter()
	// Set a timeout value on the request context (ctx), that will signal through
	// ctx.Done() that the request has timed out and further processing should be
	// stopped.
	router.Use(middleware.Timeout(30 * time.Second))

	router.Get("/v2/_catalog", proxy.Catalog)
	router.Get("/v2/{owner}/{name}/tags/list", proxy.TagsList)
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s -> %s", r.Method, r.URL, upstreamURL)
		upstreamProxy.ServeHTTP(w, r)
	})

	return &http.Server{
		Addr:    addr,
		Handler: router,
	}
}

// Catalog returns the list of repositories available in the Container Registry.
func (p *containerProxy) Catalog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Fetch the list of container packages the current (authenticated) user has
	// access to, unless `GITHUB_PACKAGES_OWNER` is set. In which case, we fetch
	// the list of container packages for this specified owner.
	opts := &github.PackageListOptions{PackageType: &packageType}
	packages, _, err := p.ghClient.ListPackages(r.Context(), p.owner, opts)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errors := makeError(ERROR_UNKNOWN, fmt.Sprintf("ListPackages: %s", err))
		json.NewEncoder(w).Encode(&errors)
		return
	}

	catalog := struct {
		Repositories []string `json:"repositories"`
	}{
		Repositories: []string{},
	}
	for _, pack := range packages {
		if pack.Name == nil || pack.Owner.Login == nil {
			continue
		}

		catalog.Repositories = append(
			catalog.Repositories,
			fmt.Sprintf("%s/%s", *pack.Owner.Login, *pack.Name),
		)
	}
	json.NewEncoder(w).Encode(catalog)
}

// TagsList returns the list of tags for a given repository.
func (p *containerProxy) TagsList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	owner := chi.URLParam(r, "owner")
	name := chi.URLParam(r, "name")

	versions, _, err := p.ghClient.PackageGetAllVersions(r.Context(), owner, packageType, name, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errors := makeError(ERROR_UNKNOWN, fmt.Sprintf("PackageGetAllVersions: %s", err))
		json.NewEncoder(w).Encode(errors)
		return
	}

	list := struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}{
		Name: fmt.Sprintf("%s/%s", owner, name),
		Tags: []string{},
	}
	for _, version := range versions {
		if version.Metadata == nil || version.Metadata.Container == nil {
			continue
		}

		list.Tags = append(
			list.Tags,
			version.Metadata.Container.Tags...,
		)
	}
	json.NewEncoder(w).Encode(list)
}

func main() {
	host := os.Getenv("HOST")
	if host == "" {
		host = defaultHost
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	rawUpstreamURL := os.Getenv("UPSTREAM_URL")
	if rawUpstreamURL == "" {
		rawUpstreamURL = defaultUpstreamURL
	}

	// Create a GitHub client to call the REST API.
	ctx := context.Background()
	client := github.NewTokenClient(ctx, os.Getenv("GITHUB_TOKEN"))

	addr := fmt.Sprintf("%s:%s", host, port)
	// We intentionally allow empty string to be the default value here, since an
	// empty owner means the GitHub API will default to the authenticated user.
	owner := os.Getenv("GITHUB_PACKAGES_OWNER")
	proxy := NewProxy(addr, client.Users, rawUpstreamURL, owner)

	log.Printf("starting container registry proxy on %s", addr)
	log.Fatal(proxy.ListenAndServe())
}
