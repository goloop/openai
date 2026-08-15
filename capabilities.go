package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/goloop/ai"
)

var _ ai.Capable = (*Client)(nil)

// Wire names of the capabilities a request can ask for. A 400 that names one
// of these, for a capability the caller actually requested, is this provider
// saying the chosen model cannot do it.
var (
	formatFeatures = []string{"response_format", "json_schema", "json_object"}
	hostedFeatures = []string{"web_search"}
)

// Capabilities describes what this driver can be asked for. It is a hint for
// the decision taken before a call - whether to offer a feature, and whether
// it needs one request or two - and never a substitute for handling
// [ai.ErrNoHosted], [ai.ErrNoFormat] or [ai.ErrFormatWithHosted], because
// support also depends on the model, the account and the region.
func (c *Client) Capabilities() ai.Capabilities {
	native := ai.FormatCapability{
		JSON:       ai.FormatNative,
		JSONSchema: ai.FormatNative,
		Strict:     ai.FormatNative,
	}

	return ai.Capabilities{
		Format: native,
		// GenerateImage draws through the images endpoint (gpt-image, dall-e).
		Images: true,
		Hosted: map[ai.HostedKind]ai.HostedCapability{
			ai.HostedWebSearch: {
				Web: &ai.HostedWebCapability{
					// The responses endpoint filters by allowed domains only,
					// and offers no ceiling on how many searches run.
					MaxUses:         false,
					AllowDomains:    true,
					BlockDomains:    false,
					BothDomainLists: false,
					Region:          true,
				},
				Policy: ai.HostedPolicyCapability{Required: true},
				// The responses endpoint carries the format too, so a search
				// and a schema fit in one call.
				WithFormat: native,
			},
		},
	}
}

// wrapUnsupportedCapability translates a provider refusal into the sentinel
// that means the same thing.
//
// A driver knows some limitations in advance and reports them before the
// request leaves. Others belong to the model rather than the provider, and
// only the provider can report them - as a 400 with a sentence in it. Without
// this, every application matches that sentence itself, which is brittle by
// construction: wordings differ between models, change between API versions,
// and when the match silently stops working the fallback stops running and the
// user gets a confidently invented answer instead of a researched one.
//
// The rules are deliberately narrow, because the cost of a false positive is
// retrying a request that was genuinely wrong: only a 400, only a capability
// the caller actually asked for, and only an error that names that exact
// feature. The original error is wrapped rather than replaced, so
// errors.As still reaches the provider's own [ai.APIError].
func wrapUnsupportedCapability(req *ai.Request, err error) error {
	var apiErr *ai.APIError
	if req == nil || !errors.As(err, &apiErr) ||
		apiErr.Status != http.StatusBadRequest {
		return err
	}

	if len(req.Hosted) > 0 && namesFeature(apiErr, hostedFeatures) {
		return fmt.Errorf("%w: %w", ai.ErrNoHosted, err)
	}
	if req.Format != nil && req.Format.Type != ai.FormatText &&
		namesFeature(apiErr, formatFeatures) {
		return fmt.Errorf("%w: %w", ai.ErrNoFormat, err)
	}

	return err
}

// unsupportedWording lists the ways a provider says a capability is not
// available. It is only ever consulted after the error has already been shown
// to concern a capability the caller asked for, because on its own a phrase
// like this appears in plenty of errors that are the caller's own fault.
func unsupportedWording(msg string) bool {
	for _, phrase := range []string{
		"not supported", "unsupported", "does not support",
		"not available", "not enabled", "invalid value",
	} {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}

// namesFeature reports whether a provider error is about one of the given wire
// features.
//
// Structured fields come first: they survive rewording and localisation, which
// prose does not. The message is a fallback, and it has to both name the exact
// feature and say it is unavailable - a 400 that merely mentions the feature
// while complaining about something else is a real bad request, and turning
// that into a capability refusal would send the caller retrying forever.
func namesFeature(e *ai.APIError, features []string) bool {
	if len(features) == 0 {
		return false
	}

	fields := []string{e.Code, e.Type, errorParam(e)}
	for _, f := range features {
		for _, field := range fields {
			if field != "" && strings.Contains(field, f) {
				return true
			}
		}
	}

	msg := strings.ToLower(e.Message)
	for _, f := range features {
		if strings.Contains(msg, strings.ToLower(f)) {
			return unsupportedWording(msg)
		}
	}
	return false
}

// errorParam pulls the parameter a provider blamed out of its error body,
// where it reports one. It is the most reliable signal available: the provider
// names the field of the request it rejected.
func errorParam(e *ai.APIError) string {
	if len(e.Raw) == 0 {
		return ""
	}
	var w struct {
		Error struct {
			Param string `json:"param"`
		} `json:"error"`
		Param string `json:"param"`
	}
	if json.Unmarshal(e.Raw, &w) != nil {
		return ""
	}
	if w.Error.Param != "" {
		return w.Error.Param
	}
	return w.Param
}
