package slack

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brigadecore/brigade-slack-gateway/internal/slack"
	"github.com/stretchr/testify/require"
)

func TestNewSignatureVerificationFilter(t *testing.T) {
	const testAppID = "42"
	testSecret := []byte("foobar")
	testConfig := SignatureVerificationFilterConfig{
		SlackApps: map[string]slack.App{
			testAppID: {
				AppID:            testAppID,
				AppSigningSecret: string(testSecret),
			},
		},
	}
	filter, ok :=
		NewSignatureVerificationFilter(testConfig).(*signatureVerificationFilter)
	require.True(t, ok)
	require.Equal(t, testConfig, filter.config)
}

func TestSignatureVerificationFilter(t *testing.T) {
	const testAppID = "42"
	testAppSigningSecret := []byte("foobar")
	testFilter := &signatureVerificationFilter{
		config: SignatureVerificationFilterConfig{
			SlackApps: map[string]slack.App{
				testAppID: {
					AppID:            testAppID,
					AppSigningSecret: string(testAppSigningSecret),
				},
			},
		},
	}
	testCases := []struct {
		name       string
		setup      func() *http.Request
		assertions func(handlerCalled bool, r *http.Response)
	}{
		{
			name: "signature cannot be verified",
			setup: func() *http.Request {
				bodyBytes := []byte("mr body")
				req, err :=
					http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(bodyBytes))
				require.NoError(t, err)
				// This is just a completely made up signature
				req.Header.Add("X-Slack-Signature", "johnhancock")
				return req
			},
			assertions: func(handlerCalled bool, r *http.Response) {
				require.Equal(t, http.StatusForbidden, r.StatusCode)
				require.False(t, handlerCalled)
			},
		},
		{
			name: "signature can be verified",
			setup: func() *http.Request {
				bodyBytes := []byte("api_app_id=42")
				req, err :=
					http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(bodyBytes))
				require.NoError(t, err)
				// This doesn't have to be a real timestamp as long as things match
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				timeStamp := "noon"
				req.Header.Add("X-Slack-Request-Timestamp", timeStamp)
				// Compute the signature
				hasher := hmac.New(sha256.New, testAppSigningSecret)
				_, err = hasher.Write(
					[]byte(
						fmt.Sprintf(
							"v0:%s:%s",
							timeStamp,
							string(bodyBytes),
						),
					),
				)
				require.NoError(t, err)
				// Add the signature to the request
				req.Header.Add(
					"X-Slack-Signature",
					fmt.Sprintf("v0=%x", hasher.Sum(nil)),
				)
				return req
			},
			assertions: func(handlerCalled bool, r *http.Response) {
				require.Equal(t, http.StatusOK, r.StatusCode)
				require.True(t, handlerCalled)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := testCase.setup()
			handlerCalled := false
			testFilter.Decorate(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})(rr, req)
			res := rr.Result()
			defer res.Body.Close()
			testCase.assertions(handlerCalled, res)
		})
	}
}
