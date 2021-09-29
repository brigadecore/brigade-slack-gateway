package main

// nolint: lll
import (
	"encoding/json"
	"io/ioutil"

	"github.com/brigadecore/brigade-foundations/file"
	"github.com/brigadecore/brigade-foundations/http"
	"github.com/brigadecore/brigade-foundations/os"
	libSlack "github.com/brigadecore/brigade-slack-gateway/internal/slack"
	"github.com/brigadecore/brigade-slack-gateway/receiver/internal/slack"
	"github.com/brigadecore/brigade/sdk/v2/restmachinery"
	"github.com/pkg/errors"
)

// apiClientConfig populates the Brigade SDK's APIClientOptions from
// environment variables.
func apiClientConfig() (string, string, restmachinery.APIClientOptions, error) {
	opts := restmachinery.APIClientOptions{}
	address, err := os.GetRequiredEnvVar("API_ADDRESS")
	if err != nil {
		return address, "", opts, err
	}
	token, err := os.GetRequiredEnvVar("API_TOKEN")
	if err != nil {
		return address, token, opts, err
	}
	opts.AllowInsecureConnections, err =
		os.GetBoolFromEnvVar("API_IGNORE_CERT_WARNINGS", false)
	return address, token, opts, err
}

func signatureVerificationFilterConfig() (
	slack.SignatureVerificationFilterConfig,
	error,
) {
	config := slack.SignatureVerificationFilterConfig{
		SlackApps: map[string]libSlack.App{},
	}
	slackAppsPath, err := os.GetRequiredEnvVar("SLACK_APPS_PATH")
	if err != nil {
		return config, err
	}
	var exists bool
	if exists, err = file.Exists(slackAppsPath); err != nil {
		return config, err
	}
	if !exists {
		return config, errors.Errorf("file %s does not exist", slackAppsPath)
	}
	slackAppsBytes, err := ioutil.ReadFile(slackAppsPath)
	if err != nil {
		return config, err
	}
	slackApps := []libSlack.App{}
	if err := json.Unmarshal(slackAppsBytes, &slackApps); err != nil {
		return config, err
	}
	for _, slackApp := range slackApps {
		config.SlackApps[slackApp.AppID] = slackApp
	}
	return config, nil
}

// serverConfig populates configuration for the HTTP/S server from environment
// variables.
func serverConfig() (http.ServerConfig, error) {
	config := http.ServerConfig{}
	var err error
	config.Port, err = os.GetIntFromEnvVar("PORT", 8080)
	if err != nil {
		return config, err
	}
	config.TLSEnabled, err = os.GetBoolFromEnvVar("TLS_ENABLED", false)
	if err != nil {
		return config, err
	}
	if config.TLSEnabled {
		config.TLSCertPath, err = os.GetRequiredEnvVar("TLS_CERT_PATH")
		if err != nil {
			return config, err
		}
		config.TLSKeyPath, err = os.GetRequiredEnvVar("TLS_KEY_PATH")
		if err != nil {
			return config, err
		}
	}
	return config, nil
}
