package slack

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNewSlashCommandHandler(t *testing.T) {
	s := NewSlashCommandHandler(&slashCommandService{})
	require.NotNil(t, s.(*slashCommandHandler).service)
}

func TestNewSlashCommandHandlerServeHTTP(t *testing.T) {
	testRequest, err := http.NewRequest(
		http.MethodPost,
		"/slash-commands",
		bytes.NewBufferString("just some garbage"),
	)
	require.NoError(t, err)
	testCases := []struct {
		name       string
		handler    *slashCommandHandler
		assertions func(*http.Response)
	}{
		{
			name: "error invoking service",
			handler: &slashCommandHandler{
				service: &mockSlashCommandService{
					HandleFn: func(context.Context, SlashCommand) ([]byte, error) {
						return nil, errors.New("something went wrong")
					},
				},
			},
			assertions: func(r *http.Response) {
				require.Equal(t, http.StatusInternalServerError, r.StatusCode)
			},
		},
		{
			name: "success",
			handler: &slashCommandHandler{
				service: &mockSlashCommandService{
					HandleFn: func(context.Context, SlashCommand) ([]byte, error) {
						return []byte("success"), nil
					},
				},
			},
			assertions: func(r *http.Response) {
				require.Equal(t, http.StatusOK, r.StatusCode)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			testCase.handler.ServeHTTP(rr, testRequest)
			testCase.assertions(rr.Result()) // nolint: bodyclose
		})
	}
}

type mockSlashCommandService struct {
	HandleFn func(context.Context, SlashCommand) ([]byte, error)
}

func (m *mockSlashCommandService) Handle(
	ctx context.Context,
	command SlashCommand,
) ([]byte, error) {
	return m.HandleFn(ctx, command)
}
