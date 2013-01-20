package main

import (
	"errors"
)

// gitProvider, used to make process exceptions per provider
type gitProvider struct {
	name string
	host string
}

var (
	gitProviderByHost = make(map[string]*gitProvider)
	gitProviderGithub = newGitProvider("GitHub", "github.com")

	errorNoProvider = errors.New("There is no provider for given hostname")
)

func newGitProvider(name string, host string) *gitProvider {
	gp := &gitProvider{
		name: name,
		host: host,
	}
	gitProviderByHost[gp.host] = gp
	return gp
}

func gitProviderFromHost(host string) (*gitProvider, error) {
	gp, exists := gitProviderByHost[host]
	if !exists {
		return nil, errorNoProvider
	}
	return gp, nil
}
