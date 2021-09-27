package slack

// App encapsulates the details of a Slack App that sends webbooks to this
// gateway.
type App struct {
	// AppID specifies the ID of the Slack App.
	AppID string `json:"appID"`
	// AppSigningSecret is the secret and verify requests.
	AppSigningSecret string `json:"appSigningSecret"`
	// APIToken is the bearer token that may be used by this gateway to send
	// messages to Slack.
	APIToken string `json:"apiToken"`
}
