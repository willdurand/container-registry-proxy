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

var packageType = "container"

type containerProxy struct {
	ghClient *github.Client
}

// NewProxy returns an instance of container proxy, which implements the Docker
// Registry HTTP API V2.
func NewProxy(addr string, ghClient *github.Client) *http.Server {
	proxy := containerProxy{
		ghClient: ghClient,
	}

	// Create an upstream (reverse) proxy to handle the requests not supported by
	// the container proxy.
	rawUpstreamURL := os.Getenv("UPSTREAM_URL")
	if rawUpstreamURL == "" {
		rawUpstreamURL = defaultUpstreamURL
	}
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
	// Fetch the list of container packages the current user has access to.
	opts := &github.PackageListOptions{PackageType: &packageType}
	packages, _, err := p.ghClient.Users.ListPackages(r.Context(), "", opts)

	if err != nil {
		http.Error(w, fmt.Sprintf("ListPackages: %s", err), http.StatusBadRequest)
		return
	}

	catalog := struct {
		Repositories []string `json:"repositories"`
	}{}
	for _, pack := range packages {
		catalog.Repositories = append(
			catalog.Repositories,
			fmt.Sprintf("%s/%s", *pack.Owner.Login, *pack.Name),
		)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(catalog)
}

// TagsList returns the list of tags for a given repository.
func (p *containerProxy) TagsList(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	name := chi.URLParam(r, "name")
	versions, _, err := p.ghClient.Users.PackageGetAllVersions(r.Context(), owner, packageType, name, nil)

	if err != nil {
		http.Error(w, fmt.Sprintf("PackageGetAllVersions: %s", err), http.StatusBadRequest)
		return
	}

	list := struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}{
		Name: name,
	}
	for _, version := range versions {
		list.Tags = append(
			list.Tags,
			version.Metadata.Container.Tags...,
		)
	}
	w.Header().Set("Content-Type", "application/json")
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
	addr := fmt.Sprintf("%s:%s", host, port)

	ctx := context.Background()
	// Create a GitHub client to call the REST API.
	client := github.NewTokenClient(ctx, os.Getenv("GITHUB_TOKEN"))

	proxy := NewProxy(addr, client)

	log.Printf("starting container registry proxy on %s", addr)
	log.Fatal(proxy.ListenAndServe())
}
