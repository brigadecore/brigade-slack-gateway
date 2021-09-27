package slack

import (
	"net/http"
)

// slashCommandHandler is an implementation of the http.Handler interface that
// can handle slash commands from Slack by delegating to a transport-agnostic
// Service interface.
type slashCommandHandler struct {
	service SlashCommandService
}

// NewSlashCommandHandler returns an implementation of the http.Handler
// interface that can handle slash commands from Slack by delegating to a
// transport-agnostic Service interface.
func NewSlashCommandHandler(service SlashCommandService) http.Handler {
	return &slashCommandHandler{
		service: service,
	}
}

func (s *slashCommandHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	command := SlashCommand{
		TeamID:         r.FormValue("team_id"),
		TeamDomain:     r.FormValue("team_domain"),
		EnterpriseID:   r.FormValue("enterprise_id"),
		EnterpriseName: r.FormValue("enterprise_name"),
		ChannelID:      r.FormValue("channel_id"),
		ChannelName:    r.FormValue("channel_name"),
		UserID:         r.FormValue("user_id"),
		Command:        r.FormValue("command"),
		Text:           r.FormValue("text"),
		ResponseURL:    r.FormValue("response_url"),
		TriggerID:      r.FormValue("trigger_id"),
		APIAppID:       r.FormValue("api_app_id"),
	}
	response, err := s.service.Handle(r.Context(), command)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"status": "internal server error"}`)) // nolint: errcheck
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(response) // nolint: errcheck
}
