package client

import "fmt"

// Environment selects a Dakota Platform deployment target.
type Environment string

const (
	// EnvironmentSandbox is the safe default for all SDK clients.
	EnvironmentSandbox Environment = "sandbox"
	// EnvironmentProduction targets live production infrastructure.
	EnvironmentProduction Environment = "production"
	// EnvironmentDevelopment targets the shared development environment.
	EnvironmentDevelopment Environment = "development"
	// EnvironmentLocal targets local development instances.
	EnvironmentLocal Environment = "local"
)

const (
	productionBaseURL  = "https://api.platform.dakota.xyz"
	sandboxBaseURL     = "https://api.platform.sandbox.dakota.xyz"
	developmentBaseURL = "https://api.platform.dev.dakota.xyz"
	localBaseURL       = "http://localhost:6464"
)

// BaseURL returns the base URL for an environment.
func (e Environment) BaseURL() (string, error) {
	switch e {
	case "", EnvironmentSandbox:
		return sandboxBaseURL, nil
	case EnvironmentProduction:
		return productionBaseURL, nil
	case EnvironmentDevelopment:
		return developmentBaseURL, nil
	case EnvironmentLocal:
		return localBaseURL, nil
	default:
		return "", fmt.Errorf("unknown environment %q", e)
	}
}
