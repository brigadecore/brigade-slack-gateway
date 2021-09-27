package slack

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"

	libHTTP "github.com/brigadecore/brigade-foundations/http"
	"github.com/brigadecore/brigade-slack-gateway/internal/slack"
)

// SignatureVerificationFilterConfig encapsulates configuration for the
// signature verification based auth filter.
type SignatureVerificationFilterConfig struct {
	// SlackApps is a map of Slack App configurations indexed by App ID.
	SlackApps map[string]slack.App
}

// signatureVerificationFilter is a component that implements the http.Filter
// interface and can conditionally allow or disallow a request based on the
// ability to verify the signature of the inbound request.
type signatureVerificationFilter struct {
	config SignatureVerificationFilterConfig
}

// NewSignatureVerificationFilter returns a component that implements the
// http.Filter interface and can conditionally allow or disallow a request based
// on the ability to verify the signature of the inbound request.
func NewSignatureVerificationFilter(
	config SignatureVerificationFilterConfig,
) libHTTP.Filter {
	return &signatureVerificationFilter{
		config: config,
	}
}

func (s *signatureVerificationFilter) Decorate(
	handle http.HandlerFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If there is no request body, fail right away or else we'll be staring
		// down the barrel of a nil pointer dereference.
		if r.Body == nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// If we encounter an error reading the request body, we're just going to
		// roll with it. The empty request body will naturally make the signature
		// verification algorithm fail.
		bodyBytes, _ := ioutil.ReadAll(r.Body) // nolint: errcheck
		r.Body.Close()                         // nolint: errcheck
		// Replace the request body because the original read was destructive!
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		appID := r.FormValue("api_app_id")

		// Now compute the signature...
		hasher := hmac.New(
			sha256.New,
			[]byte(s.config.SlackApps[appID].AppSigningSecret),
		)
		// Again, we're just going to roll with whatever errors may have occurred
		// here and let the algorithm fail to verify the signature.
		hasher.Write( // nolint: errcheck
			[]byte(
				fmt.Sprintf(
					"v0:%s:%s",
					r.Header.Get("X-Slack-Request-Timestamp"),
					string(bodyBytes),
				),
			),
		)
		computedSignature := fmt.Sprintf("v0=%x", hasher.Sum(nil))

		// If the computed signature does not match the signature provided with
		// the request, return a 403.
		if computedSignature != r.Header.Get("X-Slack-Signature") {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// If we get this far, everything checks out. Handle the request.
		handle(w, r)
	}
}
