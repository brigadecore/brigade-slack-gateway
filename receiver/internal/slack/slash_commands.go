package slack

// SlashCommand encapsulates details of a Slack slash command.
//
// nolint: lll
type SlashCommand struct {
	TeamID         string `json:"teamID"`         // e.g. T0001
	TeamDomain     string `json:"teamDomain"`     // e.g. example
	EnterpriseID   string `json:"enterpriseID"`   // e.g. E0001
	EnterpriseName string `json:"enterpriseName"` // e.g. Globular%20Construct%20Inc
	ChannelID      string `json:"channelID"`      // e.g. C2147483705
	ChannelName    string `json:"channelName"`    // e.g. test
	UserID         string `json:"userID"`         // e.g. U2147483697
	Command        string `json:"command"`        // e.g. /weather
	Text           string `json:"text"`           // e.g. 94070
	ResponseURL    string `json:"responseURL"`    // e.g. https://hooks.slack.com/commands/1234/5678
	TriggerID      string `json:"triggerID"`      // e.g. 13345224609.738474920.8088930838d88f008e0
	APIAppID       string `json:"apiAppID"`       // e.g. A123456
}
