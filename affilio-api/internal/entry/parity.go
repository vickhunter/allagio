package entry

// ScreenAction is one action or number on the setup page, the dashboard, or
// the digest. Part B wraps each row's endpoint. A handler test fails when a
// row's method and path are not mounted.
type ScreenAction struct {
	Screen      string
	Action      string
	Method      string
	Path        string
	SessionOnly bool // true when a project key or an account key must receive 403
}

// ScreenActions is the parity contract for the setup page, the dashboard, and
// the digest. Numbers are rows too: the screen reads them from the endpoint.
var ScreenActions = []ScreenAction{
	{Screen: "setup", Action: "Read the saved answers", Method: "GET", Path: "/v1/setup/settings"},
	{Screen: "setup", Action: "Number: preselected mode", Method: "GET", Path: "/v1/setup/settings"},
	{Screen: "setup", Action: "Save maker answers (what to promote, topics to avoid, commission, mode)", Method: "PUT", Path: "/v1/setup/settings"},
	{Screen: "setup", Action: "Save publisher answers (sites and channels, topics never promoted, box position, mode)", Method: "PUT", Path: "/v1/setup/settings"},
	{Screen: "setup", Action: "Approve the device code", Method: "POST", Path: "/v1/device/approve"},
	{Screen: "setup", Action: "Deny the device code", Method: "POST", Path: "/v1/device/approve"},

	{Screen: "dashboard", Action: "List sites and offers with their mode", Method: "GET", Path: "/v1/account/projects"},
	{Screen: "dashboard", Action: "Number: mode on each project", Method: "GET", Path: "/v1/account/projects"},
	{Screen: "dashboard", Action: "Read a site policy", Method: "GET", Path: "/v1/sites/:id/mode"},
	{Screen: "dashboard", Action: "Number: minimum relevance", Method: "GET", Path: "/v1/sites/:id/mode"},
	{Screen: "dashboard", Action: "Number: maximum new placements per 30 days", Method: "GET", Path: "/v1/sites/:id/mode"},
	{Screen: "dashboard", Action: "Save a site mode and policy", Method: "PUT", Path: "/v1/sites/:id/mode"},
	{Screen: "dashboard", Action: "Loosen the approval mode (manual to auto, auto to yolo, or manual to yolo)", Method: "PUT", Path: "/v1/sites/:id/mode", SessionOnly: true},
	{Screen: "dashboard", Action: "Raise a cap or lower minimum relevance", Method: "PUT", Path: "/v1/sites/:id/mode", SessionOnly: true},
	{Screen: "dashboard", Action: "Read an offer policy", Method: "GET", Path: "/v1/offers/:id/mode"},
	{Screen: "dashboard", Action: "Number: minimum site quality", Method: "GET", Path: "/v1/offers/:id/mode"},
	{Screen: "dashboard", Action: "Save an offer mode and policy", Method: "PUT", Path: "/v1/offers/:id/mode"},
	{Screen: "dashboard", Action: "Turn the kill switch on", Method: "POST", Path: "/v1/account/pause"},
	{Screen: "dashboard", Action: "Turn the kill switch off", Method: "POST", Path: "/v1/account/resume", SessionOnly: true},
	{Screen: "dashboard", Action: "Show the hard checks", Method: "GET", Path: "/v1/policy/hard-checks"},
	{Screen: "dashboard", Action: "Create an account-wide key", Method: "POST", Path: "/v1/account-keys", SessionOnly: true},
	{Screen: "dashboard", Action: "List account keys", Method: "GET", Path: "/v1/account-keys"},
	{Screen: "dashboard", Action: "Revoke an account key", Method: "POST", Path: "/v1/account-keys/:id/revoke"},
	{Screen: "dashboard", Action: "Record dashboard opened", Method: "POST", Path: "/v1/events"},
	{Screen: "dashboard", Action: "Number: went live", Method: "GET", Path: "/v1/account/digest"},
	{Screen: "dashboard", Action: "Number: clicks", Method: "GET", Path: "/v1/account/digest"},
	{Screen: "dashboard", Action: "Number: earnings", Method: "GET", Path: "/v1/account/digest"},
	{Screen: "dashboard", Action: "Number: clicks by source", Method: "GET", Path: "/v1/account/digest"},

	{Screen: "digest", Action: "Account totals (went live, clicks, earnings, clicks by source)", Method: "GET", Path: "/v1/account/digest"},
	{Screen: "digest", Action: "Per project went live, clicks, earnings, clicks by source, and an undo token per item", Method: "GET", Path: "/v1/account/digest"},
	{Screen: "digest", Action: "Undo a placement as the publisher or the maker", Method: "POST", Path: "/v1/placements/:id/revoke"},
	{Screen: "digest", Action: "Restore a placement within 6 seconds of its undo", Method: "POST", Path: "/v1/placements/:id/restore"},
	{Screen: "digest", Action: "Plugin acknowledges a removal or a re-insert", Method: "POST", Path: "/v1/placements/:id/plugin-ack"},
	{Screen: "setup", Action: "Session signed in and human login", Method: "GET", Path: "/v1/setup/settings"},
	{Screen: "setup", Action: "Preselected mode from the install path", Method: "GET", Path: "/v1/setup/settings"},
	{Screen: "digest", Action: "Session signed in", Method: "GET", Path: "/v1/account/digest"},
	{Screen: "dashboard", Action: "Approval policy rule id and sentence", Method: "POST", Path: "/v1/placements/:id/decide"},
	{Screen: "digest", Action: "Record digest opened", Method: "POST", Path: "/v1/events"},
	{Screen: "digest", Action: "List what went live since the last digest open", Method: "GET", Path: "/v1/digest"},
}
