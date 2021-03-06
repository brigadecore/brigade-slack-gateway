package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/brigadecore/brigade-slack-gateway/internal/slack"
	"github.com/brigadecore/brigade/sdk/v3"
	"github.com/brigadecore/brigade/sdk/v3/meta"
	sdkTesting "github.com/brigadecore/brigade/sdk/v3/testing"
	"github.com/stretchr/testify/require"
)

func TestMonitorEvents(t *testing.T) {
	testCases := []struct {
		name       string
		monitor    *monitor
		assertions func(error)
	}{
		{
			name: "error listing events",
			monitor: &monitor{
				config: monitorConfig{
					listEventsInterval: time.Second,
				},
				eventsClient: &sdkTesting.MockEventsClient{
					ListFn: func(
						context.Context,
						*sdk.EventsSelector,
						*meta.ListOptions,
					) (sdk.EventList, error) {
						return sdk.EventList{}, errors.New("something went wrong")
					},
				},
			},
			assertions: func(err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "something went wrong")
				require.Contains(t, err.Error(), "error listing events")
			},
		},
		{
			name: "success",
			monitor: &monitor{
				config: monitorConfig{
					listEventsInterval: time.Second,
				},
				eventsClient: &sdkTesting.MockEventsClient{
					ListFn: func(
						context.Context,
						*sdk.EventsSelector,
						*meta.ListOptions,
					) (sdk.EventList, error) {
						return sdk.EventList{
							Items: []sdk.Event{
								{
									ObjectMeta: meta.ObjectMeta{
										ID: "tunguska",
									},
								},
							},
						}, nil
					},
				},
				reportEventStatusFn: func(sdk.Event) error {
					return nil
				},
			},
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			testCase.monitor.errCh = make(chan error)
			go testCase.monitor.monitorEvents(ctx)
			// Listen for errors
			select {
			case err := <-testCase.monitor.errCh:
				testCase.assertions(err)
			case <-ctx.Done():
				testCase.assertions(nil)
			}
			cancel()
		})
	}
}

func TestMonitorReportEventStatus(t *testing.T) {
	testCases := []struct {
		name       string
		monitor    *monitor
		event      sdk.Event
		assertions func(error)
	}{
		{
			name:    "appID label missing",
			monitor: &monitor{},
			event:   sdk.Event{},
			assertions: func(err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "no slack app ID found in event")
			},
		},
		{
			name: "no config found for appID",
			monitor: &monitor{
				config: monitorConfig{},
			},
			event: sdk.Event{
				Qualifiers: map[string]string{
					"appID": "42",
				},
			},
			assertions: func(err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "no configuration found for app ID")
			},
		},
		{
			name: "error rendering status message",
			monitor: &monitor{
				config: monitorConfig{
					slackApps: map[string]slack.App{
						"42": {},
					},
				},
				prepareEventStatusMessageFn: func(sdk.Event) (*bytes.Buffer, error) {
					return nil, errors.New("something went wrong")
				},
			},
			event: sdk.Event{
				Qualifiers: map[string]string{
					"appID": "42",
				},
			},
			assertions: func(err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "something went wrong")
				require.Contains(
					t,
					err.Error(),
					"error rendering status message for for event",
				)
			},
		},
		{
			name: "error sending message",
			monitor: &monitor{
				config: monitorConfig{
					slackApps: map[string]slack.App{
						"42": {},
					},
				},
				prepareEventStatusMessageFn: func(sdk.Event) (*bytes.Buffer, error) {
					return bytes.NewBufferString("this is a status message"), nil
				},
				httpSendFn: func(*http.Request) (*http.Response, error) {
					return nil, errors.New("something went wrong")
				},
			},
			event: sdk.Event{
				Qualifiers: map[string]string{
					"appID": "42",
				},
			},
			assertions: func(err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "something went wrong")
				require.Contains(
					t,
					err.Error(),
					"error sending slack status message for event",
				)
			},
		},
		{
			name: "non-200 response when sending message",
			monitor: &monitor{
				config: monitorConfig{
					slackApps: map[string]slack.App{
						"42": {},
					},
				},
				prepareEventStatusMessageFn: func(sdk.Event) (*bytes.Buffer, error) {
					return bytes.NewBufferString("this is a status message"), nil
				},
				httpSendFn: func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
					}, nil
				},
			},
			event: sdk.Event{
				Qualifiers: map[string]string{
					"appID": "42",
				},
			},
			assertions: func(err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"error sending slack status message for event",
				)
				require.Contains(t, err.Error(), "received status code")
			},
		},
		{
			name: "error updating source state",
			monitor: &monitor{
				config: monitorConfig{
					slackApps: map[string]slack.App{
						"42": {},
					},
				},
				prepareEventStatusMessageFn: func(sdk.Event) (*bytes.Buffer, error) {
					return bytes.NewBufferString("this is a status message"), nil
				},
				httpSendFn: func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
					}, nil
				},
				eventsClient: &sdkTesting.MockEventsClient{
					UpdateSourceStateFn: func(
						context.Context,
						string,
						sdk.SourceState,
						*sdk.EventSourceStateUpdateOptions,
					) error {
						return errors.New("something went wrong")
					},
				},
			},
			event: sdk.Event{
				Qualifiers: map[string]string{
					"appID": "42",
				},
			},
			assertions: func(err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "something went wrong")
				require.Contains(
					t,
					err.Error(),
					"error clearing source state for event",
				)
			},
		},
		{
			name: "success",
			monitor: &monitor{
				config: monitorConfig{
					slackApps: map[string]slack.App{
						"42": {},
					},
				},
				prepareEventStatusMessageFn: func(sdk.Event) (*bytes.Buffer, error) {
					return bytes.NewBufferString("this is a status message"), nil
				},
				httpSendFn: func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
					}, nil
				},
				eventsClient: &sdkTesting.MockEventsClient{
					UpdateSourceStateFn: func(
						context.Context,
						string,
						sdk.SourceState,
						*sdk.EventSourceStateUpdateOptions,
					) error {
						return nil
					},
				},
			},
			event: sdk.Event{
				Qualifiers: map[string]string{
					"appID": "42",
				},
			},
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.assertions(testCase.monitor.reportEventStatus(testCase.event))
		})
	}
}

func TestMonitorPrepareStatusMessage(t *testing.T) {
	testEvent := sdk.Event{
		ObjectMeta: meta.ObjectMeta{
			ID: "123456789",
		},
		ProjectID: "italian",
		Labels: map[string]string{
			"channelID": "hbo",
		},
		Worker: &sdk.Worker{
			Status: sdk.WorkerStatus{
				Phase: sdk.WorkerPhaseSucceeded,
			},
		},
		Summary: "It worked!",
	}
	monitor := &monitor{}
	var err error
	monitor.statusMsgTemplate, err = template.New(
		"template",
	).Funcs(sprig.TxtFuncMap()).Parse(statusMsgTemplate)
	require.NoError(t, err)
	buffer, err := monitor.prepareEventStatusMessage(testEvent)
	require.NoError(t, err)
	require.NotNil(t, buffer)
	require.Contains(t, buffer.String(), testEvent.Labels["channelID"])
	require.Contains(t, buffer.String(), "Event Status Update")
	require.Contains(t, buffer.String(), testEvent.ID)
	require.Contains(t, buffer.String(), testEvent.ProjectID)
	require.Contains(t, buffer.String(), testEvent.Worker.Status.Phase)
	require.Contains(t, buffer.String(), testEvent.Summary)
}
