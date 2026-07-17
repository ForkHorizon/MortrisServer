package httpapi

import (
	"strings"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

func badRequest(err error) error {
	return apierr.New(400, contracts.CodeInvalidRequest, err.Error())
}

// decodeErr classifies a strict JSON decode failure the same way
// internal/contracts' own (unexported) classifier does — duplicated
// rather than exported, since it's three lines and exporting an
// internal-only helper across packages isn't worth the coupling.
func decodeErr(err error) error {
	code := contracts.CodeInvalidRequest
	if strings.Contains(err.Error(), "unknown field") {
		code = contracts.CodeUnknownField
	}
	return apierr.New(400, code, err.Error())
}
