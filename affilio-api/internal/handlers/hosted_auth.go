package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

type HostedHandler struct {
	DB              *database.DB
	BaseURL         string
	LoopbackURL     string
	Login           services.OwnerLogin
	Limiter         *IPRateLimiter
	Payments        *MerchantConnectionsHandler
	Page            *template.Template
	AssetsPath      string
	DownloadsPath   string
	OperatorID      string
	Assessments     *AssessmentsHandler
	DemoCheckoutURL string
	ClaimsCheck     string
	EventLimiter    *IPRateLimiter
	// TrustedProxies is the TRUSTED_PROXIES list. Empty trusts no proxy.
	TrustedProxies string
	// Render is RENDER=true. The landing event limit then uses the client
	// address Render appended, not the socket address of Render's proxy.
	Render bool
	// TrustedPlatform is a dedicated client-IP header name. When set, that
	// header is the event-limit key and Gin's TrustedPlatform.
	TrustedPlatform string

	pluginZipMu     sync.Mutex
	pluginZipKey    string
	pluginZipBody   []byte
	pluginZipBuilds int
}

func ValidatePublicURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" || u.Scheme != "https" && !(u.Scheme == "http" && loopbackHost(u.Hostname())) {
		return errors.New("PUBLIC_BASE_URL must be an HTTPS origin or an HTTP loopback origin")
	}
	return nil
}

func loopbackHost(host string) bool { return host == "127.0.0.1" || host == "::1" }

// loopbackRedirectHost is the RFC 8252 loopback set for native-app callbacks:
// the numeric addresses plus the localhost name. Public base URLs stay on
// loopbackHost so a hosted origin is still an HTTPS or numeric loopback URL.
func loopbackRedirectHost(host string) bool {
	return loopbackHost(host) || strings.EqualFold(host, "localhost")
}
func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
func (h *HostedHandler) resource() string { return h.BaseURL + "/mcp" }
func (h *HostedHandler) secure() bool     { return strings.HasPrefix(h.BaseURL, "https://") }
func ownerCookieName(baseURL string) string {
	if strings.HasPrefix(baseURL, "https://") {
		return "__Host-dtr_owner"
	}
	return "dtr_owner"
}

func (h *HostedHandler) cookieName() string {
	return ownerCookieName(h.BaseURL)
}
func (h *HostedHandler) loginCookieName() string {
	if h.secure() {
		return "__Host-dtr_login"
	}
	return "dtr_login"
}
func (h *HostedHandler) cookie(c *gin.Context, name, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: h.secure(), SameSite: http.SameSiteLaxMode})
}

func (h *HostedHandler) Mount(r *gin.Engine) {
	if h.EventLimiter == nil {
		h.EventLimiter = NewIPRateLimiter(60, time.Minute)
	}
	// Gin trusts every proxy until this runs. An empty list trusts none, so a
	// client cannot pick a new rate-limit key with X-Forwarded-For. Render
	// traffic does not use that list: eventClientIP reads the address Render
	// appended, or TrustedPlatform when that header is configured.
	if err := middleware.ConfigureTrustedProxies(r, h.TrustedProxies); err != nil {
		_ = middleware.ConfigureTrustedProxies(r, "")
	}
	if header := strings.TrimSpace(h.TrustedPlatform); header != "" {
		r.TrustedPlatform = header
	}
	public := r.Group("", h.headers)
	public.POST("/api/events", h.RecordLandingEvent)
	h.mountDemo(public)
	public.GET("/.well-known/oauth-protected-resource", h.resourceMetadata)
	public.GET("/.well-known/oauth-protected-resource/mcp", h.resourceMetadata)
	public.GET("/.well-known/oauth-authorization-server", h.authorizationMetadata)
	public.POST("/oauth/register", h.RegisterOAuthClient)
	public.GET("/oauth/authorize", h.Authorize)
	public.POST("/oauth/token", h.Token)
	public.GET("/auth/login", h.LoginStart)
	public.GET("/auth/callback", h.LoginCallback)
	public.GET("/device", h.DevicePage)
	public.POST("/device", h.ownerSession, h.DeviceDecision)
	owner := public.Group("/owner", h.ownerSession)
	owner.GET("", h.Dashboard)
	owner.POST("/accounts", h.OwnerAccount)
	owner.POST("/logout", h.Logout)
	owner.POST("/authorize", h.Consent)
	owner.GET("/connect-site", h.ReviewSiteConnection)
	owner.POST("/connect-site", h.CompleteSiteConnection)
	h.mountWorkspace(public, owner)
	owner.POST("/grants/:id/revoke", h.RevokeGrant)
	owner.GET("/placements/:id", h.ReviewPlacement)
	owner.POST("/placements/:id/approve", h.ApprovePlacement)
	owner.POST("/placements/:id/revoke", h.requirePublisher, h.RevokePlacement)
	owner.POST("/payouts/:id/confirm-received", h.requirePublisher, h.ConfirmReceived)
	if h.Payments != nil {
		owner.GET("/products/:id/connection", h.Payments.Form)
		owner.POST("/products/:id/connection", h.Payments.Connect)
	}
	public.Any("/mcp", h.MCP)
	// Gin redirects an extra trailing slash before group middleware, and it
	// refuses a second /privacy/ route, so /privacy// never reaches headers().
	// Serve a cleaned /privacy or /terms here. Other routes keep a one-slash redirect.
	r.RedirectTrailingSlash = false
	r.NoRoute(func(c *gin.Context) { h.unmatched(c, r) })
}

func (h *HostedHandler) unmatched(c *gin.Context, r *gin.Engine) {
	requestPath := c.Request.URL.Path
	if (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead) && strings.HasSuffix(path.Clean(requestPath), ".md") && landingMiss(requestPath) {
		c.Data(http.StatusNotFound, "text/markdown; charset=utf-8", []byte("not found\n"))
		return
	}
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		cleaned := path.Clean(requestPath)
		if h.AssetsPath != "" && strings.HasPrefix(cleaned, "/protocol/") {
			h.writePublicHeaders(c)
			h.Protocol(c)
			return
		}
		switch cleaned {
		case "/privacy":
			if h.AssetsPath != "" {
				h.writePublicHeaders(c)
				h.Privacy(c)
				return
			}
		case "/terms":
			if h.AssetsPath != "" {
				h.writePublicHeaders(c)
				h.Terms(c)
				return
			}
		case "/landing-v9":
			if h.AssetsPath != "" {
				h.writePublicHeaders(c)
				h.writeBuiltHTML(c, "not-found")
				return
			}
		}
	}
	if redirectTrailingSlash(c, r) {
		return
	}
	// A cleaned draft path that is not a page, such as /docs/../privacy/index.html,
	// stays a 404. It still sends noindex while the wording is unapproved.
	if !LEGAL_PAGES_APPROVED && draftLegalPath(requestPath) {
		c.Header("X-Robots-Tag", "noindex")
	}
	if !landingMiss(requestPath) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	h.writePublicHeaders(c)
	h.writeLandingNotFound(c)
}

// InstallSlashRedirect turns off Gin's built-in trailing-slash redirect and
// replaces it. The API process calls this when the hosted handler is not
// mounted, so /v1/go/<id>/ still 301s with the go-link cache headers.
func InstallSlashRedirect(r *gin.Engine) {
	r.RedirectTrailingSlash = false
	r.NoRoute(func(c *gin.Context) {
		if redirectTrailingSlash(c, r) {
			return
		}
		c.String(http.StatusNotFound, "404 page not found")
	})
}

func redirectTrailingSlash(c *gin.Context, r *gin.Engine) bool {
	requestPath := c.Request.URL.Path
	target, ok := trailingSlashTarget(c.Request.Method, requestPath, r)
	if !ok {
		return false
	}
	if !LEGAL_PAGES_APPROVED && (draftLegalPath(requestPath) || draftLegalPath(target)) {
		c.Header("X-Robots-Tag", "noindex")
	}
	if target == "/v1/go" || strings.HasPrefix(target, "/v1/go/") {
		stampGoResponse(c)
	}
	if q := c.Request.URL.RawQuery; q != "" {
		target += "?" + q
	}
	status := http.StatusTemporaryRedirect
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		status = http.StatusMovedPermanently
	}
	c.Redirect(status, target)
	return true
}

func trailingSlashTarget(method, requestPath string, r *gin.Engine) (string, bool) {
	if len(requestPath) < 2 || !strings.HasPrefix(requestPath, "/") || strings.HasPrefix(requestPath, "//") || !singleSlashPath(requestPath) {
		return "", false
	}
	alt := requestPath + "/"
	if requestPath[len(requestPath)-1] == '/' {
		alt = requestPath[:len(requestPath)-1]
	}
	if !singleSlashPath(alt) {
		return "", false
	}
	for _, route := range r.Routes() {
		if route.Method == method && (route.Path == alt || routePatternMatches(route.Path, alt)) {
			return alt, true
		}
	}
	return "", false
}

// singleSlashPath accepts /v1/go/<id> and a route that is only registered with
// one trailing slash. It rejects a protocol-relative target, an empty segment,
// and a . or .. segment.
func singleSlashPath(requestPath string) bool {
	if requestPath == "" || !strings.HasPrefix(requestPath, "/") || strings.HasPrefix(requestPath, "//") {
		return false
	}
	cleaned := path.Clean(requestPath)
	if requestPath != cleaned && requestPath != cleaned+"/" {
		return false
	}
	for _, seg := range strings.Split(strings.Trim(requestPath, "/"), "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// routePatternMatches reports whether requestPath fits a Gin route with a
// :name segment. A catch-all *filepath is not a match: a missing static file
// re-enters this handler, and treating it as a slash redirect loops.
func routePatternMatches(pattern, requestPath string) bool {
	if strings.Contains(pattern, "*") || !strings.Contains(pattern, ":") {
		return false
	}
	patternSegs := strings.Split(strings.Trim(pattern, "/"), "/")
	requestSegs := strings.Split(strings.Trim(requestPath, "/"), "/")
	for i, seg := range patternSegs {
		if i >= len(requestSegs) || requestSegs[i] == "" || requestSegs[i] == "." || requestSegs[i] == ".." {
			return false
		}
		if strings.HasPrefix(seg, ":") {
			continue
		}
		if seg != requestSegs[i] {
			return false
		}
	}
	return len(patternSegs) == len(requestSegs)
}

func (h *HostedHandler) headers(c *gin.Context) {
	h.writePublicHeaders(c)
	c.Next()
}

func (h *HostedHandler) writePublicHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "strict-origin")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; connect-src 'self'; img-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	if !LEGAL_PAGES_APPROVED && draftLegalPath(c.Request.URL.Path) {
		c.Header("X-Robots-Tag", "noindex")
	}
}

func (h *HostedHandler) limited(c *gin.Context) bool {
	if h.Limiter != nil && !h.Limiter.Allow(c.ClientIP()) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return true
	}
	return false
}

func (h *HostedHandler) currentOwner(c *gin.Context) (*database.Owner, error) {
	token, err := c.Cookie(h.cookieName())
	if err != nil {
		return nil, err
	}
	return h.DB.OwnerSession(c.Request.Context(), services.HashAPIKey(token))
}

func (h *HostedHandler) ownerSession(c *gin.Context) {
	o, err := h.currentOwner(c)
	if err != nil {
		if c.Request.Method == http.MethodGet {
			c.Redirect(http.StatusFound, "/auth/login?return_to="+url.QueryEscape(c.Request.URL.RequestURI()))
			c.Abort()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sign in at /auth/login"})
		return
	}
	if c.Request.Method != http.MethodGet {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32768)
		csrf := c.GetHeader("X-CSRF-Token")
		if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
			if c.Request.ParseForm() != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			csrf = c.PostForm("csrf")
		}
		if c.GetHeader("Origin") != h.BaseURL || subtle.ConstantTimeCompare([]byte(csrf), []byte(o.CSRF)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid owner confirmation"})
			return
		}
	}
	c.Set("owner", o)
	if o.SiteID != "" {
		c.Set("site_id", o.SiteID)
	}
	if o.MakerID != "" {
		c.Set("maker_id", o.MakerID)
	}
	c.Next()
}

func (h *HostedHandler) RequireOwnerSession(c *gin.Context) { h.ownerSession(c) }

func ownerFrom(c *gin.Context) *database.Owner { return c.MustGet("owner").(*database.Owner) }

func (h *HostedHandler) requirePublisher(c *gin.Context) {
	if ownerFrom(c).SiteID == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Next()
}

func (h *HostedHandler) LoginStart(c *gin.Context) {
	if h.limited(c) {
		return
	}
	returnTo := c.DefaultQuery("return_to", "/owner")
	u, err := url.Parse(returnTo)
	validPath := u != nil && (u.Path == "/owner" || u.Path == "/owner/products" || u.Path == "/owner/placements" || u.Path == "/owner/earnings" || u.Path == "/owner/settings" || u.Path == "/owner/review" || u.Path == "/owner/connect-site" || u.Path == "/oauth/authorize" || deviceSignInReturn(u) || strings.HasPrefix(u.Path, "/owner/placements/") && uuidPattern.MatchString(strings.TrimPrefix(u.Path, "/owner/placements/")))
	if err != nil || !validPath || u.IsAbs() || u.Host != "" || u.Fragment != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid return path"})
		return
	}
	if deviceSignInReturn(u) {
		returnTo = deviceReturnPath(u.Query().Get("user_code"))
	} else {
		returnTo = mergeAttributionSource(returnTo, c.Query("source"))
		returnTo = mergeDemoReferral(returnTo, c.Query("ref"), c.Query("via"))
	}
	if len(returnTo) > 8192 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid return path"})
		return
	}
	state, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	nonce, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	verifier, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	if err = h.DB.SaveLoginAttempt(c.Request.Context(), services.HashAPIKey(state), database.LoginAttempt{Nonce: nonce, Verifier: verifier, ReturnTo: returnTo}); err != nil {
		c.Status(500)
		return
	}
	h.cookie(c, h.loginCookieName(), state, 600)
	c.Redirect(http.StatusFound, h.Login.AuthorizationURL(state, nonce, verifier))
}

func (h *HostedHandler) LoginCallback(c *gin.Context) {
	if h.limited(c) {
		return
	}
	state := c.Query("state")
	cookie, err := c.Cookie(h.loginCookieName())
	h.cookie(c, h.loginCookieName(), "", -1)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie)) != 1 || c.Query("code") == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sign-in state mismatch"})
		return
	}
	a, err := h.DB.ConsumeLoginAttempt(c.Request.Context(), services.HashAPIKey(state))
	if err != nil {
		c.JSON(400, gin.H{"error": "sign-in attempt expired"})
		return
	}
	o, err := h.Login.Verify(c.Request.Context(), c.Query("code"), a.Nonce, a.Verifier)
	if err != nil {
		c.JSON(401, gin.H{"error": "sign-in identity could not be verified"})
		return
	}
	token, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	csrf, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	o.Source = attributionSourceFromReturn(a.ReturnTo)
	if err = h.DB.StartOwnerSession(c.Request.Context(), o, services.HashAPIKey(token), csrf); err != nil {
		c.Status(500)
		return
	}
	h.cookie(c, h.cookieName(), token, 8*3600)
	c.Redirect(http.StatusFound, a.ReturnTo)
}

func (h *HostedHandler) Logout(c *gin.Context) {
	token, _ := c.Cookie(h.cookieName())
	if err := h.DB.EndOwnerSession(c.Request.Context(), services.HashAPIKey(token)); err != nil {
		c.Status(500)
		return
	}
	h.cookie(c, h.cookieName(), "", -1)
	c.Redirect(http.StatusSeeOther, "/auth/login")
}

func (h *HostedHandler) resourceMetadata(c *gin.Context) {
	c.JSON(200, gin.H{"resource": h.resource(), "authorization_servers": []string{h.BaseURL}, "scopes_supported": []string{"publisher", "maker"}, "bearer_methods_supported": []string{"header"}})
}

func (h *HostedHandler) authorizationMetadata(c *gin.Context) {
	c.JSON(200, gin.H{"issuer": h.BaseURL, "authorization_endpoint": h.BaseURL + "/oauth/authorize", "token_endpoint": h.BaseURL + "/oauth/token", "registration_endpoint": h.BaseURL + "/oauth/register", "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"none"}, "scopes_supported": []string{"publisher", "maker"}})
}

func validRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || len(raw) > 2048 {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http" && loopbackRedirectHost(u.Hostname())
}

// redirectRegistered matches an authorization redirect to a registered URI.
// HTTPS callbacks match exactly. An http loopback callback may use any port:
// the registered host, path, and query stay fixed (RFC 8252 section 7.3).
func redirectRegistered(registered []string, raw string) bool {
	if slices.Contains(registered, raw) {
		return true
	}
	got, ok := loopbackRedirect(raw)
	if !ok {
		return false
	}
	for _, item := range registered {
		want, ok := loopbackRedirect(item)
		if ok && want == got {
			return true
		}
	}
	return false
}

type loopbackRedirectKey struct {
	host, path, rawQuery string
}

func loopbackRedirect(raw string) (loopbackRedirectKey, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.Fragment != "" || !loopbackRedirectHost(u.Hostname()) {
		return loopbackRedirectKey{}, false
	}
	return loopbackRedirectKey{host: strings.ToLower(u.Hostname()), path: u.EscapedPath(), rawQuery: u.RawQuery}, true
}

func (h *HostedHandler) RegisterOAuthClient(c *gin.Context) {
	if h.limited(c) {
		return
	}
	var req struct {
		Name          string   `json:"client_name"`
		RedirectURIs  []string `json:"redirect_uris"`
		AuthMethod    string   `json:"token_endpoint_auth_method"`
		GrantTypes    []string `json:"grant_types"`
		ResponseTypes []string `json:"response_types"`
	}
	if bindBoundedJSON(c, &req) != nil || len(req.Name) > 80 || len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > 5 || req.AuthMethod != "" && req.AuthMethod != "none" {
		c.JSON(400, gin.H{"error": "invalid_client_metadata"})
		return
	}
	for _, u := range req.RedirectURIs {
		if !validRedirect(u) {
			c.JSON(400, gin.H{"error": "invalid_redirect_uri"})
			return
		}
	}
	for _, g := range req.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			c.JSON(400, gin.H{"error": "invalid_client_metadata"})
			return
		}
	}
	for _, r := range req.ResponseTypes {
		if r != "code" {
			c.JSON(400, gin.H{"error": "invalid_client_metadata"})
			return
		}
	}
	id, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "MCP client"
	}
	if err = h.DB.SaveOAuthClient(c.Request.Context(), database.OAuthClient{ID: id, Name: req.Name, RedirectURIs: req.RedirectURIs}); err != nil {
		c.Status(500)
		return
	}
	c.JSON(201, gin.H{"client_id": id, "client_name": req.Name, "redirect_uris": req.RedirectURIs, "token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}})
}

func normalizeScope(raw string) (string, bool) {
	fields := strings.Fields(raw)
	if len(fields) == 0 || len(fields) > 2 {
		return "", false
	}
	slices.Sort(fields)
	fields = slices.Compact(fields)
	for _, f := range fields {
		if f != "publisher" && f != "maker" {
			return "", false
		}
	}
	return strings.Join(fields, " "), true
}

func (h *HostedHandler) Authorize(c *gin.Context) {
	if h.limited(c) {
		return
	}
	client, err := h.DB.OAuthClient(c.Request.Context(), c.Query("client_id"))
	if err != nil || !redirectRegistered(client.RedirectURIs, c.Query("redirect_uri")) {
		c.JSON(400, gin.H{"error": "invalid_client_or_redirect"})
		return
	}
	scope, ok := normalizeScope(c.Query("scope"))
	challenge := c.Query("code_challenge")
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(challenge)
	if c.Query("scope") != "" && !ok || c.Query("response_type") != "code" || c.Query("resource") != h.resource() || c.Query("code_challenge_method") != "S256" || decodeErr != nil || len(decoded) != 32 || len(c.Query("state")) > 2048 {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}
	o, err := h.currentOwner(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login?return_to="+url.QueryEscape(c.Request.URL.RequestURI()))
		return
	}
	if scope == "" {
		scope = "publisher"
		if o.SiteID == "" && o.MakerID != "" {
			scope = "maker"
		}
	}
	if scope == "maker" && o.MakerID == "" {
		o, err = h.ensureMakerAccount(c, o)
		if err != nil {
			c.Status(500)
			return
		}
	}
	if strings.Contains(scope, "publisher") && o.SiteID == "" || strings.Contains(scope, "maker") && o.MakerID == "" {
		h.render(c, "Connect your account", "Choose the account your agent will use.", template.HTML(fmt.Sprintf(`<p><a href="/owner?return_to=%s">Set up your account and continue</a></p>`, template.HTMLEscapeString(url.QueryEscape(c.Request.URL.RequestURI())))))
		return
	}
	id, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	a := database.OAuthRequest{ID: id, OwnerID: o.ID, ClientID: client.ID, RedirectURI: c.Query("redirect_uri"), Resource: h.resource(), Scope: scope, Challenge: challenge, State: c.Query("state")}
	if err = h.DB.SaveOAuthRequest(c.Request.Context(), a); err != nil {
		c.Status(500)
		return
	}
	h.render(c, "Connect your agent", "", template.HTML(fmt.Sprintf(`<p>Allow <strong>%s</strong> to use <strong>%s</strong> tools for seven days?</p><p>The client supplies its name. Its callback is <code>%s</code>.</p><p>Publisher agents can prepare placements, commit approved placements and read earnings. Maker agents can manage products and draft offers. Publishing approval, payment connections and receipt of money require your separate confirmation.</p><form method="post" action="/owner/authorize"><input type="hidden" name="csrf" value="%s"><input type="hidden" name="request_id" value="%s"><button>Allow access</button></form><p><a href="/owner">Cancel</a></p>`, template.HTMLEscapeString(client.Name), template.HTMLEscapeString(scope), template.HTMLEscapeString(a.RedirectURI), template.HTMLEscapeString(o.CSRF), id)))
}

func (h *HostedHandler) Consent(c *gin.Context) {
	code, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	a, err := h.DB.AuthorizeAgent(c.Request.Context(), c.PostForm("request_id"), ownerFrom(c).ID, services.HashAPIKey(code))
	if err != nil {
		c.JSON(409, gin.H{"error": "authorization request unavailable"})
		return
	}
	u, _ := url.Parse(a.RedirectURI)
	q := u.Query()
	q.Set("code", code)
	if a.State != "" {
		q.Set("state", a.State)
	}
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusSeeOther, u.String())
}

func (h *HostedHandler) Token(c *gin.Context) {
	if h.limited(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	if c.Request.ParseForm() != nil || c.PostForm("resource") != h.resource() {
		c.JSON(400, gin.H{"error": "invalid_target"})
		return
	}
	access, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	refresh, err := randomToken()
	if err != nil {
		c.Status(500)
		return
	}
	var scope string
	switch c.PostForm("grant_type") {
	case "authorization_code":
		verifier := c.PostForm("code_verifier")
		if !validVerifier(verifier) {
			c.JSON(400, gin.H{"error": "invalid_grant"})
			return
		}
		scope, err = h.DB.ExchangeAgentCode(c.Request.Context(), services.HashAPIKey(c.PostForm("code")), c.PostForm("client_id"), c.PostForm("redirect_uri"), h.resource(), oauth2.S256ChallengeFromVerifier(verifier), services.HashAPIKey(access), services.HashAPIKey(refresh))
	case "refresh_token":
		scope, err = h.DB.RefreshAgentToken(c.Request.Context(), services.HashAPIKey(c.PostForm("refresh_token")), c.PostForm("client_id"), h.resource(), services.HashAPIKey(access), services.HashAPIKey(refresh))
	default:
		c.JSON(400, gin.H{"error": "unsupported_grant_type"})
		return
	}
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid_grant"})
		return
	}
	grant, err := h.DB.AgentToken(c.Request.Context(), services.HashAPIKey(access), h.resource())
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid_grant"})
		return
	}
	c.JSON(200, gin.H{"access_token": access, "token_type": "Bearer", "expires_in": max(0, int(time.Until(grant.ExpiresAt)/time.Second)), "refresh_token": refresh, "scope": scope})
}

func validVerifier(v string) bool {
	if len(v) < 43 || len(v) > 128 {
		return false
	}
	for _, r := range v {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~", r) {
			return false
		}
	}
	return true
}

func (h *HostedHandler) RevokeGrant(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.Status(400)
		return
	}
	ok, err := h.DB.RevokeOwnerGrant(c.Request.Context(), ownerFrom(c).ID, c.Param("id"))
	if err != nil {
		c.Status(500)
		return
	}
	if !ok {
		c.Status(404)
		return
	}
	c.Redirect(303, "/owner")
}

func validAuthorizationReturn(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && !u.IsAbs() && u.Host == "" && u.Path == "/oauth/authorize" && u.Fragment == "" && len(raw) <= 8192
}

func (h *HostedHandler) RevokePlacement(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.Status(404)
		return
	}
	ok, err := h.DB.RevokePlacement(c.Request.Context(), ownerFrom(c).SiteID, c.Param("id"))
	if err != nil {
		c.Status(500)
		return
	}
	if !ok {
		c.Status(404)
		return
	}
	c.Redirect(303, "/owner/placements/"+c.Param("id"))
}

func (h *HostedHandler) ConfirmReceived(c *gin.Context) {
	if !uuidPattern.MatchString(c.Param("id")) {
		c.Status(404)
		return
	}
	owner := ownerFrom(c)
	if h.holdDashboardStep(c, database.StepPayout, owner.ID, owner.SiteID, map[string]any{"action": "confirm_received", "payout_id": c.Param("id")}) {
		return
	}
	ok, err := h.DB.ConfirmPayoutReceived(c.Request.Context(), owner.SiteID, c.Param("id"))
	if err != nil {
		c.Status(500)
		return
	}
	if !ok {
		c.JSON(409, gin.H{"error": "payment is not awaiting receipt confirmation"})
		return
	}
	c.Redirect(303, "/owner")
}

func (h *HostedHandler) OwnerAccount(c *gin.Context) {
	o := ownerFrom(c)
	role := c.PostForm("role")
	key := c.PostForm("api_key")
	if role != "maker" && role != "publisher" {
		c.Status(400)
		return
	}
	if key != "" {
		var accountID string
		if role == "publisher" {
			site, err := h.DB.GetSiteByAnyKeyHash(c.Request.Context(), services.HashAPIKey(key), services.KeyPrefix(key))
			if err != nil {
				c.JSON(401, gin.H{"error": "valid publisher owner key required"})
				return
			}
			accountID = site.ID
		} else {
			maker, err := h.DB.GetMakerByAnyAPIKey(c.Request.Context(), key)
			if err != nil {
				c.JSON(401, gin.H{"error": "valid maker owner key required"})
				return
			}
			accountID = maker.ID
		}
		ok, err := h.DB.BindOwnerAccount(c.Request.Context(), o.ID, role, accountID)
		if err != nil || !ok {
			c.JSON(409, gin.H{"error": "account already linked"})
			return
		}
	} else {
		domain := normalizeDomain(c.PostForm("domain"))
		if role == "publisher" && !validDomain(domain) {
			c.JSON(400, gin.H{"error": "valid publisher domain required"})
			return
		}
		if role == "publisher" && h.holdDashboardStep(c, database.StepSiteSignup, o.ID, o.SiteID, map[string]any{
			"kind": "owner_account", "role": "publisher", "domain": domain, "owner_id": o.ID, "name": o.Name,
		}) {
			return
		}
		key, err := randomToken()
		if err != nil {
			c.Status(500)
			return
		}
		if err = h.DB.CreateOwnerAccount(c.Request.Context(), o, role, domain, services.HashAPIKey(key), services.KeyPrefix(key)); err != nil {
			c.JSON(409, gin.H{"error": "account unavailable; link an existing account with its owner key"})
			return
		}
	}
	next := c.PostForm("return_to")
	if !validAuthorizationReturn(next) {
		next = "/owner"
	}
	c.Redirect(303, next)
}

func (h *HostedHandler) ReviewPlacement(c *gin.Context) {
	o := ownerFrom(c)
	if o.SiteID == "" || !uuidPattern.MatchString(c.Param("id")) {
		c.Status(404)
		return
	}
	p, err := h.DB.GetPlacement(c.Request.Context(), o.SiteID, c.Param("id"))
	if err != nil {
		c.Status(404)
		return
	}
	var terms struct {
		CommissionType       string `json:"commission_type"`
		CommissionValueMinor int64  `json:"commission_value_minor"`
		Currency             string `json:"currency"`
		LandingURL           string `json:"landing_url"`
		CookieDays           int    `json:"cookie_duration_days"`
		RefundDays           int    `json:"refund_window_days"`
		Mode                 string `json:"payment_mode"`
		FeePercent           int    `json:"service_fee_percent_of_commission"`
		Text                 string `json:"terms"`
	}
	if err = json.Unmarshal(p.Terms, &terms); err != nil {
		c.Status(500)
		return
	}
	commission := fmt.Sprintf("%.2f %s per first paid checkout", float64(terms.CommissionValueMinor)/100, terms.Currency)
	if terms.CommissionType == "percentage" {
		commission = fmt.Sprintf("%.2f%% of the first paid checkout, excluding tax and shipping", float64(terms.CommissionValueMinor)/100)
	}
	summary := fmt.Sprintf("Destination: %s\n%s\nRefund hold: %d days. Attribution window: %d days.\nMerchant pays directly. Service fee: %d%% of commission.\nCurrency: %s. Payment mode: %s.\n%s", terms.LandingURL, commission, terms.RefundDays, terms.CookieDays, terms.FeePercent, terms.Currency, terms.Mode, terms.Text)
	approvedBy := ""
	if p.ApprovedBy != "" {
		approvedBy = `<p>Approved by: ` + template.HTMLEscapeString(p.ApprovedBy)
		if p.MatchedRule != "" {
			approvedBy += template.HTMLEscapeString(" · " + p.MatchedRule)
		}
		approvedBy += `</p>`
	}
	body := fmt.Sprintf(`<p>Status: %s · Expires: %s</p>%s<p><a href="%s">%s</a></p><h2>Recommendation</h2><pre>%s</pre><h2>Disclosure</h2><pre>%s</pre><h2>Terms</h2><pre>%s</pre>`, template.HTMLEscapeString(PlacementDashboardLabel(p.Status, p.RemovedAt)), p.ExpiresAt.Format(time.RFC3339), approvedBy, template.HTMLEscapeString(p.SurfaceURL), template.HTMLEscapeString(p.SurfaceURL), template.HTMLEscapeString(p.Copy), template.HTMLEscapeString(p.Disclosure), template.HTMLEscapeString(summary))
	if p.Status == "pending" && p.ExpiresAt.After(time.Now()) {
		body += fmt.Sprintf(`<form method="post" action="/owner/placements/%s/approve"><input type="hidden" name="csrf" value="%s"><button>Approve this exact placement</button></form>`, p.ID, template.HTMLEscapeString(o.CSRF))
	}
	body += `<p>Approval lets your agent create the tracked link. Publish from WordPress or your existing publishing tool.</p>`
	if p.Status != "revoked" {
		body += fmt.Sprintf(`<form method="post" action="/owner/placements/%s/revoke"><input type="hidden" name="csrf" value="%s"><button>Revoke placement</button></form>`, p.ID, template.HTMLEscapeString(o.CSRF))
	}
	body += string(h.placementReceiptHTML(p))
	if p.Status == "approved" {
		body += fmt.Sprintf(`<p><button data-commit-placement="%s">Create tracked link</button></p>`, p.ID)
	}
	body += `<div id="workspace-feedback" role="status" aria-live="polite"></div>`
	h.render(c, "Review this placement", "", template.HTML(body))
}

func (h *HostedHandler) ApprovePlacement(c *gin.Context) {
	o := ownerFrom(c)
	if o.SiteID == "" || !uuidPattern.MatchString(c.Param("id")) {
		c.Status(404)
		return
	}
	ok, err := h.DB.ApproveOwnerPlacement(c.Request.Context(), o, c.Param("id"))
	if err != nil {
		c.Status(500)
		return
	}
	if !ok {
		c.JSON(409, gin.H{"error": "placement unavailable, expired or self referral"})
		return
	}
	c.Redirect(303, "/owner/placements/"+c.Param("id"))
}

var ownerPage = template.Must(template.New("owner").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="csrf-token" content="{{.CSRF}}"><title>{{.Title}} · Offer Rails</title><style>body{margin:40px auto;max-width:760px;padding:0 20px;font:17px/1.6 system-ui;color:#172b28;background:#f7faf8}h1{line-height:1.2}h2{margin-top:32px}a{color:#006a50}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:white;padding:16px;border:1px solid #d5e0da}input,select,button{font:inherit;padding:10px;margin:6px 0;max-width:100%;box-sizing:border-box}button{background:#006a50;color:white;border:0;cursor:pointer}label{display:block}section{padding:16px;border:1px solid #d5e0da;margin:16px 0}</style><header><a href="/owner">Offer Rails</a></header><main><h1>{{.Title}}</h1><p>{{.Message}}</p>{{.Body}}</main></html>`))

func (h *HostedHandler) render(c *gin.Context, title, message string, body any) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	page := ownerPage
	if h.Page != nil {
		page = h.Page
	}
	data := map[string]any{"Title": title, "Message": message, "Body": body, "BaseURL": h.BaseURL, "CSRF": ""}
	if value, ok := c.Get("owner"); ok {
		o := value.(*database.Owner)
		data["CSRF"], data["Name"], data["OwnerID"] = o.CSRF, o.Name, o.ID
		data["Maker"], data["Publisher"], data["Operator"] = o.MakerID != "", o.SiteID != "", h.OperatorID != "" && o.ID == h.OperatorID
	}
	if err := page.Execute(c.Writer, data); err != nil {
		c.Abort()
	}
}

func (h *HostedHandler) ensureMakerAccount(c *gin.Context, o *database.Owner) (*database.Owner, error) {
	err := h.DB.CreateOwnerAccount(c.Request.Context(), o, "maker", "", "", "")
	if isUniqueViolation(err) {
		return o, nil
	}
	fresh, loadErr := h.currentOwner(c)
	if loadErr != nil {
		return nil, loadErr
	}
	if err != nil && fresh.MakerID == "" {
		return nil, err
	}
	return fresh, nil
}

func (h *HostedHandler) Dashboard(c *gin.Context) {
	o := ownerFrom(c)
	if c.Query("setup") == "maker" && o.MakerID == "" {
		var err error
		o, err = h.ensureMakerAccount(c, o)
		if err != nil {
			c.Status(500)
			return
		}
		c.Set("owner", o)
		c.Set("maker_id", o.MakerID)
	}
	if h.Page != nil {
		pageBody := `<div id="workspace-app" aria-live="polite"></div>`
		if c.Request.URL.Path == "/owner/settings" {
			section, err := h.humanApprovalSection(c, o)
			if err != nil {
				c.Status(500)
				return
			}
			pageBody = string(section) + pageBody
		}
		h.render(c, "Your distribution workspace", "", template.HTML(pageBody))
		return
	}
	csrf := template.HTMLEscapeString(o.CSRF)
	var body strings.Builder
	if c.Request.URL.Path == "/owner/settings" {
		section, err := h.humanApprovalSection(c, o)
		if err != nil {
			c.Status(500)
			return
		}
		body.WriteString(string(section))
	}
	returnTo := c.Query("return_to")
	if !validAuthorizationReturn(returnTo) {
		returnTo = ""
	}
	fmt.Fprintf(&body, `<p>Signed in as %s.</p><p>Hosted MCP URL: <code>%s</code></p>`, template.HTMLEscapeString(o.Email), template.HTMLEscapeString(h.resource()))
	for _, role := range []string{"maker", "publisher"} {
		if role == "maker" && o.MakerID != "" || role == "publisher" && o.SiteID != "" {
			continue
		}
		fmt.Fprintf(&body, `<section><h2>Set up your %s account</h2><form method="post" action="/owner/accounts"><input type="hidden" name="csrf" value="%s"><input type="hidden" name="role" value="%s">`, role, csrf, role)
		fmt.Fprintf(&body, `<input type="hidden" name="return_to" value="%s">`, template.HTMLEscapeString(returnTo))
		if role == "publisher" {
			body.WriteString(`<label>Publisher domain <input name="domain" placeholder="your-blog.com"></label>`)
		}
		body.WriteString(`<label>Existing account? Enter its owner API key <input type="password" name="api_key" autocomplete="off"></label><button>Connect account</button></form></section>`)
	}
	if o.SiteID != "" {
		placements, err := h.DB.ListPlacements(c.Request.Context(), o.SiteID)
		if err != nil {
			c.Status(500)
			return
		}
		body.WriteString(`<h2>Placement inbox</h2>`)
		for _, p := range placements {
			fmt.Fprintf(&body, `<section><a href="/owner/placements/%s">Review placement</a><p>%s · %s</p><p>%s</p></section>`, p.ID, template.HTMLEscapeString(PlacementDashboardLabel(p.Status, p.RemovedAt)), template.HTMLEscapeString(p.SurfaceURL), template.HTMLEscapeString(p.Copy))
		}
		payouts, err := h.DB.ListPayouts(c.Request.Context(), o.SiteID)
		if err != nil {
			c.Status(500)
			return
		}
		body.WriteString(`<h2>Payments</h2>`)
		payoutModes, err := h.DB.LoadStepModes(c.Request.Context(), o.ID, o.SiteID)
		if err != nil {
			c.Status(500)
			return
		}
		payoutMode := payoutModes.Get(database.StepPayout)
		for _, p := range payouts {
			fmt.Fprintf(&body, `<section>%.2f %s · %s · %s`, p.Amount, template.HTMLEscapeString(p.Currency), template.HTMLEscapeString(p.Mode), template.HTMLEscapeString(p.Status))
			if p.Status == "paid" && p.ReceivedAt == nil && p.Mode == "live" {
				if payoutMode == database.ModeOff {
					fmt.Fprintf(&body, `<p>%s</p><p data-reason="step_off">step_off</p>`, template.HTMLEscapeString(stepOffMessage(database.StepPayout)))
				} else {
					fmt.Fprintf(&body, `<form method="post" action="/owner/payouts/%s/confirm-received"><input type="hidden" name="csrf" value="%s"><button>Confirm funds arrived</button></form>`, p.ID, csrf)
					if payoutMode == database.ModeAsk {
						fmt.Fprintf(&body, `<p>%s</p>`, template.HTMLEscapeString(stepAskMessage(database.StepPayout)))
					}
				}
			}
			if p.ReceivedAt != nil {
				body.WriteString(`<p>You confirmed receipt.</p>`)
			}
			body.WriteString(`</section>`)
		}
	}
	if o.MakerID != "" {
		products, err := h.DB.ListProducts(c.Request.Context(), o.MakerID)
		if err != nil {
			c.Status(500)
			return
		}
		body.WriteString(`<h2>Products</h2>`)
		for _, p := range products {
			fmt.Fprintf(&body, `<p>%s · <a href="/owner/products/%s/connection">Connect Stripe</a></p>`, template.HTMLEscapeString(p.Name), p.ID)
		}
		makerPlacements, err := h.DB.ListMakerPlacements(c.Request.Context(), o.MakerID)
		if err != nil {
			c.Status(500)
			return
		}
		body.WriteString(`<h2>Publisher placements</h2>`)
		for _, p := range makerPlacements {
			fmt.Fprintf(&body, `<section><p>%s · %s</p><p>%s</p></section>`, template.HTMLEscapeString(PlacementDashboardLabel(p.Status, p.RemovedAt)), template.HTMLEscapeString(p.SurfaceURL), template.HTMLEscapeString(p.Copy))
		}
	}
	grants, err := h.DB.OwnerGrants(c.Request.Context(), o.ID)
	if err != nil {
		c.Status(500)
		return
	}
	body.WriteString(`<h2>Connected agents</h2>`)
	for _, g := range grants {
		fmt.Fprintf(&body, `<section>%s · %s · until %s<form method="post" action="/owner/grants/%s/revoke"><input type="hidden" name="csrf" value="%s"><button>Disconnect agent</button></form></section>`, template.HTMLEscapeString(g.ClientName), template.HTMLEscapeString(g.Scope), g.ExpiresAt.Format(time.RFC3339), g.ID, csrf)
	}
	fmt.Fprintf(&body, `<form method="post" action="/owner/logout"><input type="hidden" name="csrf" value="%s"><button>Sign out</button></form>`, csrf)
	h.render(c, "Your distribution workspace", "", template.HTML(body.String()))
}
