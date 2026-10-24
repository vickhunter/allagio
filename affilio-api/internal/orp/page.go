package orp

import (
	"html/template"
	"strings"
)

var proofPage = template.Must(template.New("proof").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Test proof</title>
<p>{{.Label}}</p>
<p>This is a test proof. The deal stays awaiting. Offer Rails does not hold money.</p>
`))

var approvePage = template.Must(template.New("approve").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Waiting for a person</title>
<p>{{.Title}}</p>
<p>{{.Label}}</p>
<p>This test deal is awaiting a person. This page does not approve it. Offer Rails does not hold money.</p>
`))

// ProofPage is the public test page a proof_url receipt checks.
// It shows the disclosure label and nothing about a person.
func ProofPage(label string) string {
	var b strings.Builder
	_ = proofPage.Execute(&b, map[string]string{"Label": label})
	return b.String()
}

// ApprovePage tells the waiting party that a person decides.
// It shows the terms title and disclosure label, and it does not approve.
func ApprovePage(title, label string) string {
	var b strings.Builder
	_ = approvePage.Execute(&b, map[string]string{"Title": title, "Label": label})
	return b.String()
}
