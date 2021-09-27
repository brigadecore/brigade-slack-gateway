package main

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/brigadecore/brigade-foundations/file"
	"github.com/brigadecore/brigade-foundations/os"
	"github.com/brigadecore/brigade-slack-gateway/internal/slack"
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

// getMonitorConfig populates configuration for the monitor from environment
// variables.
func getMonitorConfig() (monitorConfig, error) {
	config := monitorConfig{
		healthcheckInterval: 30 * time.Second,
		slackApps:           map[string]slack.App{},
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
	slackApps := []slack.App{}
	if err = json.Unmarshal(slackAppsBytes, &slackApps); err != nil {
		return config, err
	}
	for _, slackApp := range slackApps {
		config.slackApps[slackApp.AppID] = slackApp
	}
	config.listEventsInterval, err =
		os.GetDurationFromEnvVar("LIST_EVENTS_INTERVAL", 30*time.Second)
	return config, err
}
