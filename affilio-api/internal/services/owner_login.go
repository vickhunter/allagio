package services

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"affilio-api/internal/database"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OwnerLogin interface {
	AuthorizationURL(state, nonce, verifier string) string
	Verify(context.Context, string, string, string) (database.Owner, error)
}

type OIDCLogin struct {
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
	issuer   string
	client   *http.Client
}

func NewOIDCLogin(ctx context.Context, issuer, clientID, clientSecret, callbackURL string) (*OIDCLogin, error) {
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || clientID == "" || clientSecret == "" {
		return nil, errors.New("OIDC requires an HTTPS issuer, client ID and client secret")
	}
	//allagio:outbound operator-config OIDC_ISSUER
	client := &http.Client{Timeout: 10 * time.Second}
	ctx = oidc.ClientContext(ctx, client)
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	return &OIDCLogin{config: oauth2.Config{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: callbackURL, Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}, verifier: p.Verifier(&oidc.Config{ClientID: clientID}), issuer: issuer, client: client}, nil
}

func (l *OIDCLogin) AuthorizationURL(state, nonce, verifier string) string {
	return l.config.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

func (l *OIDCLogin) Verify(ctx context.Context, code, nonce, verifier string) (database.Owner, error) {
	ctx = oidc.ClientContext(ctx, l.client)
	token, err := l.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return database.Owner{}, errors.New("sign-in exchange failed")
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return database.Owner{}, errors.New("sign-in did not return an identity token")
	}
	id, err := l.verifier.Verify(ctx, raw)
	if err != nil || id.Nonce != nonce || id.Subject == "" {
		return database.Owner{}, errors.New("invalid sign-in identity")
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err = id.Claims(&claims); err != nil || !claims.EmailVerified || len(claims.Email) > 255 || !strings.Contains(claims.Email, "@") || len(claims.Name) > 255 {
		return database.Owner{}, errors.New("a verified email is required")
	}
	return database.Owner{Issuer: l.issuer, Subject: id.Subject, Email: strings.ToLower(claims.Email), Name: claims.Name}, nil
}
