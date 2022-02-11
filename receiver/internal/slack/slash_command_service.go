package slack

import (
	"bytes"
	"context"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/brigadecore/brigade/sdk/v3"
	"github.com/pkg/errors"
)

// SlashCommandService is an interface for components that can handle slash
// commands from Slack. Implementations of this interface are
// transport-agnostic.
type SlashCommandService interface {
	// Handle handles a slash command from Slack.
	Handle(context.Context, SlashCommand) ([]byte, error)
}

type slashCommandService struct {
	eventsClient   sdk.EventsClient
	ackMsgTemplate *template.Template
}

// NewSlashCommandService returns an implementation of the Service interface for
// handling slash commands from Slack.
func NewSlashCommandService(
	eventsClient sdk.EventsClient,
) (SlashCommandService, error) {
	ackMsgTemplate, err :=
		template.New("template").Funcs(sprig.TxtFuncMap()).Parse(ackMsgTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing response template")
	}
	return &slashCommandService{
		eventsClient:   eventsClient,
		ackMsgTemplate: ackMsgTemplate,
	}, nil
}

func (s *slashCommandService) Handle(
	ctx context.Context,
	command SlashCommand,
) ([]byte, error) {
	event := sdk.Event{
		Source: "brigade.sh/slack",
		Type:   command.Command[1:], // Strip the leading slash from the command
		// A workspace can have multiple apps installed that all use the same slash
		// command, so events are qualified with WHICH app produced them.
		Qualifiers: map[string]string{
			"appID": command.APIAppID,
		},
		Labels: map[string]string{
			"teamID":    command.TeamID,
			"channelID": command.ChannelID,
			"userID":    command.UserID,
		},
		SourceState: &sdk.SourceState{
			State: map[string]string{
				"tracking": "true",
			},
		},
		Payload: command.Text,
	}
	// This information is only present for Slack Enterprise Grid customers. We're
	// only including the label for those cases rather than always including it
	// and having its value often be the empty string.
	if command.EnterpriseID != "" {
		event.Labels["enterprise_id"] = command.EnterpriseID
	}
	events, err := s.eventsClient.Create(context.Background(), event, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error emitting event(s) into Brigade")
	}
	message := struct {
		Channel string
		Events  []sdk.Event
	}{
		Channel: command.ChannelID,
		Events:  events.Items,
	}
	buffer := &bytes.Buffer{}
	err = s.ackMsgTemplate.Execute(buffer, message)
	return buffer.Bytes(), errors.Wrap(err, "error rendering response")
}

var ackMsgTemplate = `{
  "response_type": "in_channel",
  "channel": {{ quote .Channel }},
  "blocks": [
    {
      "type": "header",
      "text": {
        "type": "plain_text",
        {{- if eq (len .Events) 0 }}
        "text": "No Events Created"
        {{- else }}
        "text": "Events Created for Subscribed Projects:"
        {{- end }}
      }
    },
    {{- if eq (len .Events) 0 }}
    {
      "type": "section",
      "text": {
        "type": "plain_text",
        "text": "No projects subscribed to this event."
      }
    }
    {{- else }}
    {
      "type": "section",
      "fields": [
        {
          "type": "mrkdwn",
          "text": "*Project ID*"
        },
        {
          "type": "mrkdwn",
          "text": "*Event ID*"
        }
      ]
    },
    {{ $events := .Events }}
    {{- range $index, $event := $events }}
    {
      "type": "section",
      "fields": [
        {
          "type": "plain_text",
          "text": {{ quote .ProjectID }}
        },
        {
          "type": "plain_text",
          "text": {{ quote .ID }}
        }
      ]
    }{{ if not (eq (add $index 1) (len $events)) }},{{ end }}
    {{- end }}
    {{- end }}
  ]
}`
