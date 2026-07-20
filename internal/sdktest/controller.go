// Package sdktest exposes deliberate fault scenarios only for a separately
// configured staging project. It is never enabled by the production service.
package sdktest

import (
	"crypto/subtle"
	"net/http"
	"sync"

	"github.com/ForkHorizon/Mortris/internal/contracts"
)

const (
	HeaderScenario = "X-Mortris-Test-Scenario"
	HeaderToken    = "X-Mortris-Test-Token"

	LostAcknowledgement Scenario = "lost_acknowledgement"
	UnauthorizedOnce    Scenario = "unauthorized_once"
	PayloadTooLarge     Scenario = "payload_too_large"
	RateLimited         Scenario = "rate_limited"
	PolicyActive        Scenario = "policy_active"
	PolicyPauseUpload   Scenario = "policy_pause_upload"
	PolicyDisable       Scenario = "policy_disable_collection"
)

type Scenario string

type Controller struct {
	projectID string
	token     string
	seen401   sync.Map
}

func New(projectID, token string) *Controller {
	return &Controller{projectID: projectID, token: token}
}

func (c *Controller) Scenario(r *http.Request, projectID string) Scenario {
	if c == nil || projectID != c.projectID || !matchesToken(r.Header.Get(HeaderToken), c.token) {
		return ""
	}
	return knownScenario(Scenario(r.Header.Get(HeaderScenario)))
}

func (c *Controller) FirstUnauthorized(projectID, installID string, scenario Scenario) bool {
	if c == nil || scenario != UnauthorizedOnce {
		return false
	}
	_, seen := c.seen401.LoadOrStore(projectID+"/"+installID, struct{}{})
	return !seen
}

func (s Scenario) Policy() (contracts.ClientPolicy, bool) {
	switch s {
	case PolicyActive:
		return contracts.ClientPolicy{Mode: "active", NextCheckSeconds: 300}, true
	case PolicyPauseUpload:
		return contracts.ClientPolicy{Mode: "pause_upload", NextCheckSeconds: 300}, true
	case PolicyDisable:
		return contracts.ClientPolicy{Mode: "disable_collection", NextCheckSeconds: 300, DiscardPending: true}, true
	default:
		return contracts.ClientPolicy{}, false
	}
}

func matchesToken(provided, expected string) bool {
	return len(provided) == len(expected) && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func knownScenario(s Scenario) Scenario {
	switch s {
	case LostAcknowledgement, UnauthorizedOnce, PayloadTooLarge, RateLimited, PolicyActive, PolicyPauseUpload, PolicyDisable:
		return s
	default:
		return ""
	}
}
