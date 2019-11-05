package models

// ServiceSettings - Settings from individual services annotations.
type ServiceSettings struct {
	AppName         string
	AuthURL         string
	AuthSignIn      string
	SetXAuthRequest string
	GitHub          GitHubProvider
}

// GitHubProvider - GitHub Provicer
type GitHubProvider struct {
	Organization string
	Teams        []string
}
