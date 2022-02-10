package slack

import (
	"context"
	"encoding/json"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/brigadecore/brigade/sdk/v3"
	"github.com/brigadecore/brigade/sdk/v3/meta"
	sdkTesting "github.com/brigadecore/brigade/sdk/v3/testing"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNewSlashCommandService(t *testing.T) {
	s, err := NewSlashCommandService(
		// Totally unusable client that is enough to fulfill the dependencies for
		// this test...
		&sdkTesting.MockEventsClient{
			LogsClient: &sdkTesting.MockLogsClient{},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, s.(*slashCommandService).eventsClient)
	require.NotNil(t, s.(*slashCommandService).ackMsgTemplate)
}

func TestSlashCommandServiceHandle(t *testing.T) {
	testCommand := SlashCommand{
		Command:   "/foo",
		APIAppID:  "control-app",
		TeamID:    "control",
		ChannelID: "cone-of-silence",
		UserID:    "86",
		Text:      "bar",
	}
	testCases := []struct {
		name       string
		service    *slashCommandService
		assertions func([]byte, error)
	}{
		{
			name: "error creating brigade event",
			service: &slashCommandService{
				eventsClient: &sdkTesting.MockEventsClient{
					CreateFn: func(
						context.Context,
						sdk.Event,
						*sdk.EventCreateOptions,
					) (sdk.EventList, error) {
						return sdk.EventList{}, errors.New("something went wrong")
					},
				},
			},
			assertions: func(_ []byte, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"error emitting event(s) into Brigade",
				)
				require.Contains(t, err.Error(), "something went wrong")
			},
		},
		{
			name: "success with no subscribers",
			service: &slashCommandService{
				eventsClient: &sdkTesting.MockEventsClient{
					CreateFn: func(
						_ context.Context,
						event sdk.Event,
						_ *sdk.EventCreateOptions,
					) (sdk.EventList, error) {
						require.Equal(t, "brigade.sh/slack", event.Source)
						require.Equal(t, "foo", event.Type)
						require.Equal(
							t,
							map[string]string{
								"appID": testCommand.APIAppID,
							},
							event.Qualifiers,
						)
						require.Equal(
							t,
							map[string]string{
								"teamID":    testCommand.TeamID,
								"channelID": testCommand.ChannelID,
								"userID":    testCommand.UserID,
							},
							event.Labels,
						)
						require.Equal(
							t,
							map[string]string{
								"tracking": "true",
							},
							event.SourceState.State,
						)
						require.Equal(t, "bar", event.Payload)
						return sdk.EventList{}, nil
					},
				},
			},
			assertions: func(response []byte, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, response)
				// Test that the response is valid JSON
				obj := map[string]interface{}{}
				err = json.Unmarshal(response, &obj)
				require.NoError(t, err)
				require.Contains(t, string(response), testCommand.ChannelID)
				require.Contains(t, string(response), "No Events Created")
			},
		},
		{
			name: "success with subscribers",
			service: &slashCommandService{
				eventsClient: &sdkTesting.MockEventsClient{
					CreateFn: func(
						_ context.Context,
						event sdk.Event,
						_ *sdk.EventCreateOptions,
					) (sdk.EventList, error) {
						require.Equal(t, "brigade.sh/slack", event.Source)
						require.Equal(t, "foo", event.Type)
						require.Equal(
							t,
							map[string]string{
								"appID": testCommand.APIAppID,
							},
							event.Qualifiers,
						)
						require.Equal(
							t,
							map[string]string{
								"teamID":    testCommand.TeamID,
								"channelID": testCommand.ChannelID,
								"userID":    testCommand.UserID,
							},
							event.Labels,
						)
						require.Equal(
							t,
							map[string]string{
								"tracking": "true",
							},
							event.SourceState.State,
						)
						require.Equal(t, "bar", event.Payload)
						return sdk.EventList{
							Items: []sdk.Event{
								{
									ObjectMeta: meta.ObjectMeta{
										ID: "123456789",
									},
									ProjectID: "italian",
								},
							},
						}, nil
					},
				},
			},
			assertions: func(response []byte, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, response)
				// Test that the response is valid JSON
				obj := map[string]interface{}{}
				err = json.Unmarshal(response, &obj)
				require.NoError(t, err)
				require.Contains(t, string(response), testCommand.ChannelID)
				require.Contains(
					t,
					string(response),
					"Events Created for Subscribed Projects:",
				)
				require.Contains(t, string(response), "italian")
				require.Contains(t, string(response), "123456789")
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var err error
			testCase.service.ackMsgTemplate, err = template.New(
				"template",
			).Funcs(sprig.TxtFuncMap()).Parse(ackMsgTemplate)
			require.NoError(t, err)
			response, err :=
				testCase.service.Handle(context.Background(), testCommand)
			testCase.assertions(response, err)
		})
	}
}
