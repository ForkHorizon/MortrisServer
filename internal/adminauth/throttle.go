package adminauth

import "github.com/ForkHorizon/Mortris/internal/ratelimit"

// Login throttling (section 10.3). Two independent limiters: per-email
// stops credential stuffing against one account regardless of source IP,
// per-IP stops one source hammering many accounts. Both must allow the
// attempt.
type Throttle struct {
	byEmail *ratelimit.Limiter
	byIP    *ratelimit.Limiter
}

func NewThrottle() *Throttle {
	return &Throttle{
		byEmail: ratelimit.NewLimiter(ratelimit.PerMinute(10), 10),
		byIP:    ratelimit.NewLimiter(ratelimit.PerMinute(30), 30),
	}
}

func (t *Throttle) Allow(email, sourceIP string) bool {
	return t.byEmail.Allow(email) && t.byIP.Allow(sourceIP)
}
