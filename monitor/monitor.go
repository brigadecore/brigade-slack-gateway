package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/brigadecore/brigade-slack-gateway/internal/slack"
	"github.com/brigadecore/brigade/sdk/v3"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
)

// monitorConfig encapsulates configuration options for the monitor component.
type monitorConfig struct {
	healthcheckInterval time.Duration
	listEventsInterval  time.Duration
	slackApps           map[string]slack.App
}

// monitor is a component that continuously monitors events that the Brigade
// Slack Gateway has emitted into Brigade's event bus. When each such event's
// worker reaches a terminal phase, status is reported upstream to the Slack
// channel where the event originated.
type monitor struct {
	config monitorConfig
	// All of the monitor's goroutines will send fatal errors here
	errCh chan error
	// All of these internal functions are overridable for testing purposes
	runHealthcheckLoopFn        func(context.Context)
	monitorEventsFn             func(context.Context)
	reportEventStatusFn         func(sdk.Event) error
	errFn                       func(...interface{})
	prepareEventStatusMessageFn func(sdk.Event) (*bytes.Buffer, error)
	httpSendFn                  func(*http.Request) (*http.Response, error)
	systemClient                sdk.SystemClient
	eventsClient                sdk.EventsClient
	statusMsgTemplate           *template.Template
}

// newMonitor initializes and returns a monitor.
func newMonitor(
	systemClient sdk.SystemClient,
	eventsClient sdk.EventsClient,
	config monitorConfig,
) (*monitor, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.Logger = log.New(ioutil.Discard, "", log.LstdFlags)
	statusMsgTemplate, err := template.New(
		"template",
	).Funcs(sprig.TxtFuncMap()).Parse(statusMsgTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing status template")
	}
	m := &monitor{
		config:            config,
		errCh:             make(chan error),
		httpSendFn:        retryClient.StandardClient().Do,
		statusMsgTemplate: statusMsgTemplate,
	}
	m.runHealthcheckLoopFn = m.runHealthcheckLoop
	m.monitorEventsFn = m.monitorEvents
	m.reportEventStatusFn = m.reportEventStatus
	m.errFn = log.Println
	m.prepareEventStatusMessageFn = m.prepareEventStatusMessage
	m.systemClient = systemClient
	m.eventsClient = eventsClient
	return m, nil
}

// run coordinates the many goroutines involved in different aspects of the
// monitor. If any one of these goroutines encounters an unrecoverable error,
// everything shuts down.
func (m *monitor) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg := sync.WaitGroup{}

	// Run healthcheck loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.runHealthcheckLoopFn(ctx)
	}()

	// Continuously monitor events
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.monitorEventsFn(ctx)
	}()

	// Wait for an error or a completed context
	var err error
	select {
	// If any one loop fails, including the healthcheck, shut everything else
	// down also.
	case err = <-m.errCh:
		cancel() // Shut it all down
	case <-ctx.Done():
		err = ctx.Err()
	}

	// Adapt wg to a channel that can be used in a select
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		wg.Wait()
	}()

	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		// Probably doesn't matter that this is hardcoded. Relatively speaking, 3
		// seconds is a lot of time for things to wrap up.
	}

	return err
}
