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
<p>{{.Title}}</p>
<p>{{.Label}}</p>
<p>This page does not approve it. By default an agent approves an ask with its bearer. That record is approved_by agent, not a person's approval. Offer Rails does not hold money.</p>
`))

// ProofPage is the public test page a proof_url receipt checks.
// It shows the disclosure label and nothing about a person.
// An active deal says so. A verified tracked link says so, which changes the
// page from the active line. Every other state still says the deal stays awaiting.
func ProofPage(label, state string, linkVerified bool, approvals []Approval) string {
	line := "This is a test proof. The deal stays awaiting. Offer Rails does not hold money."
	if linkVerified {
		line = "This is a test proof. The tracked link is verified. Offer Rails does not hold money."
	} else if state == "active" {
		line = "This is a test proof. The deal is active. Offer Rails does not hold money."
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
		b.WriteString(" approved_by agent. An agent approved this step.")
	}
	if owner {
		b.WriteString(" approved_by owner. The owner approved this step.")
	}
	return b.String()
}

// ApprovePage shows the terms title and disclosure label.
// By default it does not approve. An opted-in owner approves by opening the link while signed in.
func ApprovePage(title, label string) string {
	var b strings.Builder
	_ = approvePage.Execute(&b, map[string]string{"Title": title, "Label": label})
	return b.String()
}
