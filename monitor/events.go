package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/brigadecore/brigade/sdk/v3"
	"github.com/brigadecore/brigade/sdk/v3/meta"
	"github.com/pkg/errors"
)

func (m *monitor) monitorEvents(ctx context.Context) {
	ticker := time.NewTicker(m.config.listEventsInterval)
	defer ticker.Stop()
	for {
		listOpts := &meta.ListOptions{Limit: 100}
		for {
			events, err := m.eventsClient.List(
				ctx,
				&sdk.EventsSelector{
					Source: "brigade.sh/slack",
					// We only want to report back to Slack once an event's worker reaches
					// a terminal phase.
					WorkerPhases: sdk.WorkerPhasesTerminal(),
					SourceState: map[string]string{
						// Only select events that are to be tracked.
						"tracking": "true",
					},
				},
				listOpts,
			)
			if err != nil {
				select {
				case m.errCh <- errors.Wrap(err, "error listing events"):
				case <-ctx.Done():
				}
				return
			}
			for _, event := range events.Items {
				if err := m.reportEventStatusFn(event); err != nil {
					log.Println(err)
				}
			}
			if events.RemainingItemCount > 0 {
				listOpts.Continue = events.Continue
			} else {
				break
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (m *monitor) reportEventStatus(event sdk.Event) error {
	appID, ok := event.Qualifiers["appID"]
	if !ok {
		return errors.Errorf(
			"no slack app ID found in event %q qualifiers",
			event.ID,
		)
	}
	app, ok := m.config.slackApps[appID]
	if !ok {
		return errors.Errorf(
			"no configuration found for app ID %q from event %q labels",
			appID,
			event.ID,
		)
	}
	buffer, err := m.prepareEventStatusMessageFn(event)
	if err != nil {
		return errors.Wrap(err, "error rendering status message for for event %q")
	}
	req, err := http.NewRequest(
		http.MethodPost,
		"https://slack.com/api/chat.postMessage",
		buffer,
	)
	if err != nil {
		return errors.Wrapf(
			err,
			"error preparing http request with status message for event %q",
			event.ID,
		)
	}
	req.Header.Add("Content-type", "application/json")
	req.Header.Add(
		"Authorization",
		fmt.Sprintf("Bearer %s", app.APIToken),
	)
	resp, err := m.httpSendFn(req)
	if err != nil {
		return errors.Wrapf(
			err,
			"error sending slack status message for event %q",
			event.ID,
		)
	}
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf(
			"error sending slack status message for event %q: received status "+
				"code %d",
			event.ID,
			resp.StatusCode,
		)
	}
	// Blank out the Event's source state to reflect that we're done following
	// up on it
	if err := m.eventsClient.UpdateSourceState(
		context.Background(),
		event.ID,
		sdk.SourceState{},
		nil,
	); err != nil {
		return errors.Wrapf(
			err,
			"error clearing source state for event %q",
			event.ID,
		)
	}
	return nil
}

func (m *monitor) prepareEventStatusMessage(
	event sdk.Event,
) (*bytes.Buffer, error) {
	buffer := &bytes.Buffer{}
	err := m.statusMsgTemplate.Execute(buffer, event)
	return buffer, err
}

var statusMsgTemplate = `{
  "response_type": "in_channel",
  "channel": {{ quote .Labels.channelID }},
  "blocks": [
    {
      "type": "header",
      "text": {
        "type": "plain_text",
        "text": "Event Status Update"
      }
    },
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
    },
    {
      "type": "section",
      "fields": [
        {
          "type": "mrkdwn",
          "text": "*Worker Phase*"
        }
      ]
    },
    {
      "type": "section",
      "fields": [
        {
          "type": "plain_text",
          "text": {{ quote .Worker.Status.Phase }}
        }
      ]
    }{{ if .Summary }},{{ end }}
    {{- if .Summary }}
    {
      "type": "section",
      "text": {
        "type": "plain_text",
        "text": {{ quote .Summary }}
      }
    }
    {{- end }}
  ]
}`
