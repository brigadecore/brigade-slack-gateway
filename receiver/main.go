package main

import (
	"log"
	"net/http"

	libHTTP "github.com/brigadecore/brigade-foundations/http"
	"github.com/brigadecore/brigade-foundations/signals"
	"github.com/brigadecore/brigade-foundations/version"
	"github.com/brigadecore/brigade-slack-gateway/receiver/internal/slack"
	"github.com/brigadecore/brigade/sdk/v2/core"
	"github.com/gorilla/mux"
)

func main() {

	log.Printf(
		"Starting Brigade Slack Gateway Receiver -- version %s -- commit %s",
		version.Version(),
		version.Commit(),
	)

	var slashCommandsService slack.SlashCommandService
	{
		address, token, opts, err := apiClientConfig()
		if err != nil {
			log.Fatal(err)
		}
		slashCommandsService, err = slack.NewSlashCommandService(
			core.NewEventsClient(address, token, &opts),
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	var signatureVerificationFilter libHTTP.Filter
	{
		config, err := signatureVerificationFilterConfig()
		if err != nil {
			log.Fatal(err)
		}
		signatureVerificationFilter = slack.NewSignatureVerificationFilter(config)
	}

	var server libHTTP.Server
	{
		router := mux.NewRouter()
		router.StrictSlash(true)
		router.Handle(
			"/slash-commands",
			signatureVerificationFilter.Decorate(
				slack.NewSlashCommandHandler(slashCommandsService).ServeHTTP,
			),
		).Methods(http.MethodPost)
		router.HandleFunc("/healthz", libHTTP.Healthz).Methods(http.MethodGet)
		serverConfig, err := serverConfig()
		if err != nil {
			log.Fatal(err)
		}
		server = libHTTP.NewServer(router, &serverConfig)
	}

	log.Println(
		server.ListenAndServe(signals.Context()),
	)
}
