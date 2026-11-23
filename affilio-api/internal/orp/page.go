package orp

import (
	"html/template"
	"strings"
)

var proofPage = template.Must(template.New("proof").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Test proof</title>
<p>{{.Label}}</p>
<p>{{.Line}}</p>
`))

var approvePage = template.Must(template.New("approve").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Waiting for approval</title>
<p>{{.Title}}</p>
<p>{{.Label}}</p>
<p>This page does not approve it. By default an agent approves an ask with its bearer. On a 0.3 deal that call is POST /orp/deals/{deal}/activate, not this page. On an ORP 0.4 deal that call is POST /orp/approve/{escalation} with the waiting party's bearer. On an ORP 0.4 deal, GET /orp/deals/{deal} returns approved_by agent. That is not a person's approval. When the owner has ask on, this page is not the response: the signed-in owner submits the form, and that 0.4 read returns approved_by owner. A 0.3 deal does not include approved_by. Allagio does not hold money.</p>
`))

var ownerApprovePage = template.Must(template.New("owner-approve").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Waiting for approval</title>
<p>{{.Title}}</p>
<p>{{.Label}}</p>
<p>Opening this page does not approve it. Submit the form. On an ORP 0.4 deal, GET /orp/deals/{deal} returns approved_by owner. A 0.3 deal does not include approved_by.</p>
<form method="post" action="{{.Action}}">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<button>Approve</button>
</form>
`))

// ProofPage is the public test page a proof_url receipt checks.
// It shows the disclosure label and nothing about a person.
// An active deal says so. A verified tracked link says so, which changes the
// page from the active line. Every other state still says the deal stays awaiting.
func ProofPage(label, state string, linkVerified bool, approvals []Approval) string {
	line := "This is a test proof. The deal stays awaiting. Allagio does not hold money."
	if linkVerified {
		line = "This is a test proof. The tracked link is verified. Allagio does not hold money."
	} else if state == "active" {
		line = "This is a test proof. The deal is active. Allagio does not hold money."
	}
	line += approvalLine(approvals)
	var b strings.Builder
	_ = proofPage.Execute(&b, map[string]string{"Label": label, "Line": line})
	return b.String()
}

// approvalLine names who approved. An agent approval is never described as a person's.
func approvalLine(rows []Approval) string {
	agent, owner := false, false
	for _, row := range rows {
		switch row.By {
		case "agent":
			agent = true
		case "owner":
			owner = true
		}
	}
	var b strings.Builder
	if agent {
		b.WriteString(" approved_by agent. An agent approved this step. On an ORP 0.4 deal the deal read returns approved_by agent.")
	}
	if owner {
		b.WriteString(" approved_by owner. The owner approved this step. On an ORP 0.4 deal the deal read returns approved_by owner.")
	}
	return b.String()
}

// ApprovePage shows the terms title and disclosure label.
// It does not approve. The owner records approved_by owner by submitting the form.
func ApprovePage(title, label string) string {
	var b strings.Builder
	_ = approvePage.Execute(&b, map[string]string{"Title": title, "Label": label})
	return b.String()
}

// OwnerApprovePage shows the terms and a form. Opening it does not approve.
// The hidden field is the signed-in owner's confirmation token.
func OwnerApprovePage(title, label, action, csrf string) string {
	var b strings.Builder
	_ = ownerApprovePage.Execute(&b, map[string]string{
		"Title": title, "Label": label, "Action": action, "CSRF": csrf,
	})
	return b.String()
}
