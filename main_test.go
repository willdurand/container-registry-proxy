package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v50/github"
)

type githubClientMock struct {
	Packages        []*github.Package
	PackageVersions []*github.PackageVersion
	Err             error
}

func (c *githubClientMock) ListPackages(ctx context.Context, user string, opts *github.PackageListOptions) ([]*github.Package, *github.Response, error) {
	if user != "" {
		return nil, nil, fmt.Errorf("invalid user: %s", user)
	}

	return c.Packages, nil, c.Err
}

func (c *githubClientMock) PackageGetAllVersions(ctx context.Context, user, packageType, packageName string, opts *github.PackageListOptions) ([]*github.PackageVersion, *github.Response, error) {
	return c.PackageVersions, nil, c.Err
}

func TestCatalog(t *testing.T) {
	owner := &github.User{Login: github.String("some-owner")}

	for _, tc := range []struct {
		client             githubClientMock
		expectedStatusCode int
		expectedContent    string
	}{
		{
			client:             githubClientMock{},
			expectedStatusCode: 200,
			expectedContent:    `{"repositories":[]}`,
		},
		{
			client: githubClientMock{
				Packages: []*github.Package{
					{
						Name:  github.String("some-package"),
						Owner: owner,
					},
				},
			},
			expectedStatusCode: 200,
			expectedContent:    `{"repositories":["some-owner/some-package"]}`,
		},
		{
			client: githubClientMock{
				Packages: []*github.Package{
					{
						Name:  github.String("package-1"),
						Owner: owner,
					},
					{
						Name:  github.String("package-2"),
						Owner: owner,
					},
				},
			},
			expectedStatusCode: 200,
			expectedContent:    `{"repositories":["some-owner/package-1","some-owner/package-2"]}`,
		},
		{
			client: githubClientMock{
				Packages: []*github.Package{
					{
						// No Name here
						Owner: owner,
					},
				},
			},
			expectedStatusCode: 200,
			expectedContent:    `{"repositories":[]}`,
		},
		{
			client: githubClientMock{
				Packages: []*github.Package{
					{
						Name:  github.String("some-package"),
						Owner: &github.User{},
					},
				},
			},
			expectedStatusCode: 200,
			expectedContent:    `{"repositories":[]}`,
		},
		{
			client: githubClientMock{
				Err: fmt.Errorf("an error"),
			},
			expectedStatusCode: 400,
			expectedContent:    `{"errors":[{"code":"UNKNOWN","message":"ListPackages: an error","detail":""}]}`,
		},
	} {
		proxy := NewProxy(
			"127.0.0.1:10000",
			&tc.client,
			"http://127.0.0.1/upstream",
			"", // owner
		)

		req, _ := http.NewRequest("GET", "/v2/_catalog", nil)
		res := httptest.NewRecorder()
		proxy.Handler.ServeHTTP(res, req)

		if res.Code != tc.expectedStatusCode {
			t.Fatalf("expected: %d, got: %d", tc.expectedStatusCode, res.Code)
		}
		if strings.TrimSpace(res.Body.String()) != tc.expectedContent {
			t.Fatalf("expected: %s, got: %s", tc.expectedContent, res.Body.String())
		}
	}
}

func TestTagsList(t *testing.T) {
	for _, tc := range []struct {
		client             githubClientMock
		owner              string
		name               string
		expectedStatusCode int
		expectedContent    string
	}{
		{
			client:             githubClientMock{},
			owner:              "some-owner",
			name:               "some-package",
			expectedStatusCode: 200,
			expectedContent:    `{"name":"some-owner/some-package","tags":[]}`,
		},
		{
			client: githubClientMock{
				PackageVersions: []*github.PackageVersion{
					{
						Metadata: &github.PackageMetadata{
							Container: &github.PackageContainerMetadata{
								Tags: []string{"tag-1", "tag-2"},
							},
						},
					},
				},
			},
			owner:              "some-owner",
			name:               "some-package",
			expectedStatusCode: 200,
			expectedContent:    `{"name":"some-owner/some-package","tags":["tag-1","tag-2"]}`,
		},
		{
			client: githubClientMock{
				PackageVersions: []*github.PackageVersion{},
			},
			owner:              "some-owner",
			name:               "some-package",
			expectedStatusCode: 200,
			expectedContent:    `{"name":"some-owner/some-package","tags":[]}`,
		},
		{
			client: githubClientMock{
				PackageVersions: []*github.PackageVersion{
					{},
				},
			},
			owner:              "some-owner",
			name:               "some-package",
			expectedStatusCode: 200,
			expectedContent:    `{"name":"some-owner/some-package","tags":[]}`,
		},
		{
			client: githubClientMock{
				PackageVersions: []*github.PackageVersion{
					{
						Metadata: &github.PackageMetadata{},
					},
				},
			},
			owner:              "some-owner",
			name:               "some-package",
			expectedStatusCode: 200,
			expectedContent:    `{"name":"some-owner/some-package","tags":[]}`,
		},
		{
			client: githubClientMock{
				Err: fmt.Errorf("an error"),
			},
			expectedStatusCode: 400,
			expectedContent:    `{"errors":[{"code":"UNKNOWN","message":"PackageGetAllVersions: an error","detail":""}]}`,
		},
	} {
		proxy := NewProxy(
			"127.0.0.1:10000",
			&tc.client,
			"http://127.0.0.1/upstream",
			"", // owner
		)

		req, _ := http.NewRequest("GET", fmt.Sprintf("/v2/%s/%s/tags/list", tc.owner, tc.name), nil)
		res := httptest.NewRecorder()
		proxy.Handler.ServeHTTP(res, req)

		if res.Code != tc.expectedStatusCode {
			t.Fatalf("expected: %d, got: %d", tc.expectedStatusCode, res.Code)
		}
		if strings.TrimSpace(res.Body.String()) != tc.expectedContent {
			t.Fatalf("expected: %s, got: %s", tc.expectedContent, res.Body.String())
		}
	}
}

func TestCallUpstreamServer(t *testing.T) {
	upstreamResponse := "upstream server called"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s", upstreamResponse)
	}))
	defer upstream.Close()

	proxy := NewProxy(
		"127.0.0.1:10000",
		&githubClientMock{},
		upstream.URL,
		"", // owner
	)

	req, _ := http.NewRequest("GET", "/some/other/path", nil)
	res := httptest.NewRecorder()
	proxy.Handler.ServeHTTP(res, req)

	if res.Code != 200 {
		t.Fatalf("expected: %d, got: %d", 200, res.Code)
	}
	if strings.TrimSpace(res.Body.String()) != upstreamResponse {
		t.Fatalf("expected: %s, got: %s", upstreamResponse, res.Body.String())
	}
}
