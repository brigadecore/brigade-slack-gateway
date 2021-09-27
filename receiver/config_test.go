package main

// nolint: lll
import (
	"io/ioutil"
	"testing"

	"github.com/brigadecore/brigade-foundations/http"
	"github.com/brigadecore/brigade-slack-gateway/receiver/internal/slack"
	"github.com/brigadecore/brigade/sdk/v2/restmachinery"
	"github.com/stretchr/testify/require"
)

// Note that unit testing in Go does NOT clear environment variables between
// tests, which can sometimes be a pain, but it's fine here-- so each of these
// test functions uses a series of test cases that cumulatively build upon one
// another.

func TestAPIClientConfig(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func()
		assertions func(
			address string,
			token string,
			opts restmachinery.APIClientOptions,
			err error,
		)
	}{
		{
			name:  "API_ADDRESS not set",
			setup: func() {},
			assertions: func(
				_ string,
				_ string,
				_ restmachinery.APIClientOptions,
				err error,
			) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "value not found for")
				require.Contains(t, err.Error(), "API_ADDRESS")
			},
		},
		{
			name: "API_TOKEN not set",
			setup: func() {
				t.Setenv("API_ADDRESS", "foo")
			},
			assertions: func(
				_ string,
				_ string,
				_ restmachinery.APIClientOptions,
				err error,
			) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "value not found for")
				require.Contains(t, err.Error(), "API_TOKEN")
			},
		},
		{
			name: "success",
			setup: func() {
				t.Setenv("API_TOKEN", "bar")
				t.Setenv("API_IGNORE_CERT_WARNINGS", "true")
			},
			assertions: func(
				address string,
				token string,
				opts restmachinery.APIClientOptions,
				err error,
			) {
				require.NoError(t, err)
				require.Equal(t, "foo", address)
				require.Equal(t, "bar", token)
				require.True(t, opts.AllowInsecureConnections)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.setup()
			address, token, opts, err := apiClientConfig()
			testCase.assertions(address, token, opts, err)
		})
	}
}

func TestSignatureVerificationFilterConfig(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func()
		assertions func(slack.SignatureVerificationFilterConfig, error)
	}{
		{
			name: "SLACK_APPS_PATH not set",
			assertions: func(_ slack.SignatureVerificationFilterConfig, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "value not found for")
				require.Contains(t, err.Error(), "SLACK_APPS_PATH")
			},
		},
		{
			name: "SLACK_APPS_PATH path does not exist",
			setup: func() {
				t.Setenv("SLACK_APPS_PATH", "/completely/bogus/path")
			},
			assertions: func(_ slack.SignatureVerificationFilterConfig, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"file /completely/bogus/path does not exist",
				)
			},
		},
		{
			name: "SLACK_APPS_PATH does not contain valid json",
			setup: func() {
				appsFile, err := ioutil.TempFile("", "apps.json")
				require.NoError(t, err)
				defer appsFile.Close()
				_, err = appsFile.Write([]byte("this is not json"))
				require.NoError(t, err)
				t.Setenv("SLACK_APPS_PATH", appsFile.Name())
			},
			assertions: func(_ slack.SignatureVerificationFilterConfig, err error) {
				require.Error(t, err)
				require.Contains(
					t, err.Error(), "invalid character",
				)
			},
		},
		{
			name: "success",
			setup: func() {
				appsFile, err := ioutil.TempFile("", "apps.json")
				require.NoError(t, err)
				defer appsFile.Close()
				_, err =
					appsFile.Write([]byte(`[{"appID":"42","appSigningSecret":"foobar"}]`))
				require.NoError(t, err)
				t.Setenv("SLACK_APPS_PATH", appsFile.Name())
			},
			assertions: func(
				config slack.SignatureVerificationFilterConfig,
				err error,
			) {
				require.NoError(t, err)
				require.Len(t, config.SlackApps, 1)
				require.Equal(t, "42", config.SlackApps["42"].AppID)
				require.Equal(t, "foobar", config.SlackApps["42"].AppSigningSecret)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setup != nil {
				testCase.setup()
			}
			config, err := signatureVerificationFilterConfig()
			testCase.assertions(config, err)
		})
	}
}

func TestServerConfig(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func()
		assertions func(http.ServerConfig, error)
	}{
		{
			name: "PORT not an int",
			setup: func() {
				t.Setenv("PORT", "foo")
			},
			assertions: func(_ http.ServerConfig, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "was not parsable as an int")
				require.Contains(t, err.Error(), "PORT")
			},
		},
		{
			name: "TLS_ENABLED not a bool",
			setup: func() {
				t.Setenv("PORT", "8080")
				t.Setenv("TLS_ENABLED", "nope")
			},
			assertions: func(_ http.ServerConfig, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "was not parsable as a bool")
				require.Contains(t, err.Error(), "TLS_ENABLED")
			},
		},
		{
			name: "TLS_CERT_PATH required but not set",
			setup: func() {
				t.Setenv("TLS_ENABLED", "true")
			},
			assertions: func(_ http.ServerConfig, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "value not found for")
				require.Contains(t, err.Error(), "TLS_CERT_PATH")
			},
		},
		{
			name: "TLS_KEY_PATH required but not set",
			setup: func() {
				t.Setenv("TLS_CERT_PATH", "/var/ssl/cert")
			},
			assertions: func(_ http.ServerConfig, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "value not found for")
				require.Contains(t, err.Error(), "TLS_KEY_PATH")
			},
		},
		{
			name: "success",
			setup: func() {
				t.Setenv("TLS_KEY_PATH", "/var/ssl/key")
			},
			assertions: func(config http.ServerConfig, err error) {
				require.NoError(t, err)
				require.Equal(
					t,
					http.ServerConfig{
						Port:        8080,
						TLSEnabled:  true,
						TLSCertPath: "/var/ssl/cert",
						TLSKeyPath:  "/var/ssl/key",
					},
					config,
				)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.setup()
			config, err := serverConfig()
			testCase.assertions(config, err)
		})
	}
}
