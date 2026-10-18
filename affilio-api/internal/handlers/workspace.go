package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

func (h *HostedHandler) LoadWorkspace() error {
	if h.AssetsPath == "" {
		return nil
	}
	page, err := template.ParseFiles(filepath.Join(h.AssetsPath, "workspace.html"))
	if err != nil {
		return err
	}
	h.Page = page
	if h.Payments != nil {
		h.Payments.Page = page
		h.Payments.OperatorID = h.OperatorID
	}
	return nil
}

func (h *HostedHandler) mountWorkspace(public, owner *gin.RouterGroup) {
	for _, path := range []string{"/products", "/placements", "/earnings", "/settings", "/review"} {
		owner.GET(path, h.Dashboard)
	}
	api := owner.Group("/api")
	api.GET("/agents", func(c *gin.Context) {
		grants, err := h.DB.OwnerGrants(c.Request.Context(), ownerFrom(c).ID)
		if err != nil {
			c.Status(500)
			return
		}
		c.JSON(200, gin.H{"grants": grants})
	})
	api.GET("/session", func(c *gin.Context) {
		o := ownerFrom(c)
		c.JSON(200, gin.H{"name": o.Name, "email": o.Email, "owner_id": o.ID, "maker": o.MakerID != "", "publisher": o.SiteID != "", "operator": h.OperatorID != "" && o.ID == h.OperatorID, "mcp_url": h.resource(), "install_command": h.installCommand()})
	})
	products := &ProductsHandler{DB: h.DB}
	offers := &OffersHandler{DB: h.DB, ClaimsCheck: h.ClaimsCheck}
	placements := &PlacementsHandler{DB: h.DB, RequireVerifiedConnection: true}
	payouts := &PayoutsHandler{DB: h.DB}
	maker := api.Group("", h.requireMaker)
	maker.GET("/products", products.List)
	maker.POST("/products", products.Create)
	maker.PATCH("/products/:id", products.Patch)
	maker.GET("/offers", offers.List)
	maker.POST("/offers", offers.Create)
	maker.PATCH("/offers/:id", offers.Patch)
	maker.POST("/offers/:id/submit", offers.Submit)
	maker.GET("/earnings/maker", payouts.MakerEarnings)
	maker.GET("/maker-placements", placements.ListForMaker)
	maker.GET("/payments", h.MerchantPayments)
	maker.POST("/payments/:id/report", h.ReportMerchantPayment)
	if h.Assessments != nil {
		maker.POST("/products/:id/assess", h.Assessments.Assess)
		maker.GET("/products/:id/assessments", h.Assessments.List)
	}
	publisher := api.Group("", h.requirePublisher)
	publisher.POST("/opportunities", placements.Find)
	publisher.POST("/article-text", placements.ArticleText)
	publisher.POST("/placements", placements.Prepare)
	publisher.GET("/placements", placements.List)
	publisher.POST("/placements/:id/commit", placements.Commit)
	publisher.GET("/earnings/publisher", payouts.Balance)
	publisher.GET("/history", payouts.History)
	publisher.GET("/payout-profile", h.PayoutProfile)
	publisher.POST("/payout-profile", h.SavePayoutProfile)
	review := api.Group("/review", h.requireOperator)
	review.GET("/offers", (&OfferReviewHandler{DB: h.DB, ClaimsCheck: h.ClaimsCheck}).Queue)
	review.POST("/offers/:id/approve", (&OfferReviewHandler{DB: h.DB, ClaimsCheck: h.ClaimsCheck}).Approve)
	review.POST("/offers/:id/reject", (&OfferReviewHandler{DB: h.DB, ClaimsCheck: h.ClaimsCheck}).Reject)
	review.POST("/payouts/batch", payouts.RunBatch)
	if h.AssetsPath != "" {
		public.GET("/", h.Home)
		public.GET("/makers", h.Makers)
		public.GET("/makers/", h.Makers)
		public.GET("/publishers", h.Publishers)
		public.GET("/publishers/", h.Publishers)
		public.GET("/publishers/setup", h.PublisherSetup)
		public.GET("/publishers/setup/", h.PublisherSetup)
		public.GET("/makers/:slug", h.directoryProfile)
		public.GET("/makers/:slug/", h.directoryProfile)
		public.GET("/publishers/:slug", h.directoryProfile)
		public.GET("/publishers/:slug/", h.directoryProfile)
		public.GET("/protocol", h.Protocol)
		public.GET("/protocol/", h.Protocol)
		public.GET("/privacy", h.Privacy)
		public.GET("/privacy/", h.Privacy)
		public.GET("/terms", h.Terms)
		public.GET("/terms/", h.Terms)
		public.GET("/agents", h.Agents)
		public.GET("/agents/", h.Agents)
		public.Static("/landing-v9", filepath.Join(h.AssetsPath, "landing-v9"))
		for _, name := range []string{"workspace.css", "workspace.js", "setup-controls.js", "landing.css", "landing.js", "landing-scene.bundle.js", "landing-scene.bundle.js.LEGAL.txt", "THIRD_PARTY_NOTICES.md"} {
			file := name
			public.GET("/assets/"+file, func(c *gin.Context) { c.File(filepath.Join(h.AssetsPath, file)) })
		}
		public.GET("/downloads/"+installerArchive, h.serveInstaller)
		public.GET("/downloads/"+legacyInstallerArchive, h.serveInstaller)
		public.GET("/downloads/affilio.zip", h.DownloadWordPress)
		public.HEAD("/downloads/affilio.zip", h.DownloadWordPress)
	}
}

func (h *HostedHandler) requireMaker(c *gin.Context) {
	if ownerFrom(c).MakerID == "" {
		c.AbortWithStatusJSON(409, gin.H{"error": "Create your builder account first."})
		return
	}
	c.Next()
}

func (h *HostedHandler) requireOperator(c *gin.Context) {
	if h.OperatorID == "" || ownerFrom(c).ID != h.OperatorID {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.Next()
}

const (
	installerArchive       = "offer-rails-0.1.0.tgz"
	legacyInstallerArchive = "distributoor-setup-0.1.0.tgz"
)

func (h *HostedHandler) installCommand() string {
	return "npx --yes --package=" + h.BaseURL + "/downloads/" + installerArchive + " offer-rails --api-url " + h.BaseURL
}

func (h *HostedHandler) installerPath() (string, bool) {
	if h.DownloadsPath == "" {
		return "", false
	}
	primary := filepath.Join(h.DownloadsPath, installerArchive)
	if _, err := os.Stat(primary); err == nil {
		return primary, true
	}
	legacy := filepath.Join(h.DownloadsPath, legacyInstallerArchive)
	if _, err := os.Stat(legacy); err == nil {
		return legacy, true
	}
	return "", false
}

func (h *HostedHandler) serveInstaller(c *gin.Context) {
	file, ok := h.installerPath()
	if !ok {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	c.FileAttachment(file, filepath.Base(file))
}

func landingMiss(path string) bool {
	switch {
	case strings.HasPrefix(path, "/v1"),
		strings.HasPrefix(path, "/api"),
		strings.HasPrefix(path, "/mcp"),
		strings.HasPrefix(path, "/oauth"),
		strings.HasPrefix(path, "/.well-known"),
		strings.HasPrefix(path, "/downloads"),
		strings.HasPrefix(path, "/assets"),
		strings.HasPrefix(path, "/landing-v9"):
		return false
	default:
		return true
	}
}

func (h *HostedHandler) writeLandingNotFound(c *gin.Context) {
	h.writeBuiltHTML(c, "not-found")
}

func (h *HostedHandler) directoryProfile(c *gin.Context) {
	if c.Param("slug") == "setup" && strings.HasPrefix(c.Request.URL.Path, "/publishers/") {
		h.PublisherSetup(c)
		return
	}
	h.writeBuiltHTML(c, "profile")
}

func (h *HostedHandler) writeBuiltHTML(c *gin.Context, name string) {
	body := []byte(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Offer Rails · Page not found</title></head><body><header><a href="/">Offer Rails</a><nav aria-label="Main"><a href="/makers/">Makers</a><a href="/publishers/">Publishers</a><a href="/protocol/">Protocol</a></nav><a href="/auth/login?return_to=%2Fowner">Sign in</a></header><main><h1>That page is not on Offer Rails.</h1><p>The address does not match a page on this site.</p><p><a href="/">Home</a></p><p><a href="/agents">Agents</a></p></main></body></html>`)
	if h.AssetsPath != "" {
		if file, err := os.ReadFile(filepath.Join(h.AssetsPath, "landing-v9", name, "index.html")); err == nil {
			body = file
		}
	}
	c.Data(http.StatusNotFound, "text/html; charset=utf-8", body)
}

func (h *HostedHandler) Home(c *gin.Context)           { h.landing(c, "home.html") }
func (h *HostedHandler) Makers(c *gin.Context)         { h.landing(c, "makers.html") }
func (h *HostedHandler) Publishers(c *gin.Context)     { h.landing(c, "publishers.html") }
func (h *HostedHandler) PublisherSetup(c *gin.Context) { h.landing(c, "publishers-setup.html") }
func (h *HostedHandler) Protocol(c *gin.Context)       { h.landing(c, "protocol.html") }
func (h *HostedHandler) Privacy(c *gin.Context)        { h.builtPage(c, "privacy") }
func (h *HostedHandler) Terms(c *gin.Context)          { h.builtPage(c, "terms") }
func (h *HostedHandler) Agents(c *gin.Context)         { h.builtPage(c, "agents") }

// LEGAL_PAGES_APPROVED stays false until the privacy and terms wording is approved.
// Those pages send X-Robots-Tag: noindex while it is false, including the copies
// the landing asset directory serves under /landing-v9.
var LEGAL_PAGES_APPROVED = false

func draftLegalPath(requestPath string) bool {
	switch path.Clean(requestPath) {
	case "/privacy", "/privacy/index.html",
		"/terms", "/terms/index.html",
		"/landing-v9/privacy", "/landing-v9/privacy/index.html",
		"/landing-v9/terms", "/landing-v9/terms/index.html":
		return true
	default:
		return false
	}
}

// builtPage serves a Vite page from landing-v9/<name>/index.html. The file
// is sent as stored. It is not executed as a Go template, because the bundle
// contains template delimiters.
func (h *HostedHandler) builtPage(c *gin.Context, name string) {
	body, err := os.ReadFile(filepath.Join(h.AssetsPath, "landing-v9", name, "index.html"))
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(c.Writer, c.Request)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	if !LEGAL_PAGES_APPROVED && (name == "privacy" || name == "terms") {
		c.Header("X-Robots-Tag", "noindex")
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", body)
}

func (h *HostedHandler) landing(c *gin.Context, name string) {
	page, err := template.ParseFiles(filepath.Join(h.AssetsPath, name))
	if err != nil {
		c.Status(500)
		return
	}
	_, installerReady := h.installerPath()
	_, pluginErr := os.Stat(filepath.Join(h.DownloadsPath, "affilio.zip"))
	var buf bytes.Buffer
	if err = page.Execute(&buf, gin.H{"BaseURL": h.BaseURL, "InstallCommand": h.installCommand(), "InstallerReady": installerReady, "PluginReady": h.DownloadsPath != "" && pluginErr == nil}); err != nil {
		c.Abort()
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", stampLandingLinks(buf.Bytes(), landingSourceFrom(c)))
}

func (h *HostedHandler) DownloadWordPress(c *gin.Context) {
	payload, code := h.wordpressZip()
	if code != http.StatusOK {
		c.Status(code)
		return
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="affilio.zip"`)
	c.Header("Content-Length", strconv.Itoa(len(payload)))
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(payload)
}

func (h *HostedHandler) wordpressZip() ([]byte, int) {
	if h.DownloadsPath == "" {
		return nil, http.StatusServiceUnavailable
	}
	raw, err := os.ReadFile(filepath.Join(h.DownloadsPath, "affilio.zip"))
	if err != nil {
		return nil, http.StatusServiceUnavailable
	}
	sum := sha256.Sum256(raw)
	key := hex.EncodeToString(sum[:]) + "\n" + h.BaseURL
	h.pluginZipMu.Lock()
	if h.pluginZipKey == key && h.pluginZipBody != nil {
		payload := h.pluginZipBody
		h.pluginZipMu.Unlock()
		return payload, http.StatusOK
	}
	h.pluginZipMu.Unlock()
	payload, code := buildWordPressZip(raw, h.BaseURL)
	if code != http.StatusOK {
		return nil, code
	}
	h.pluginZipMu.Lock()
	if h.pluginZipKey != key {
		h.pluginZipBuilds++
	}
	h.pluginZipKey = key
	h.pluginZipBody = payload
	h.pluginZipMu.Unlock()
	return payload, http.StatusOK
}

func buildWordPressZip(raw []byte, baseURL string) ([]byte, int) {
	source, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, http.StatusServiceUnavailable
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range source.File {
		if file.UncompressedSize64 > 10<<20 {
			return nil, http.StatusInternalServerError
		}
		input, err := file.Open()
		if err != nil {
			return nil, http.StatusInternalServerError
		}
		data, err := io.ReadAll(io.LimitReader(input, 10<<20))
		input.Close()
		if err != nil {
			return nil, http.StatusInternalServerError
		}
		data = rewriteDownloadedPlugin(file.Name, data, baseURL)
		header := &zip.FileHeader{Name: file.Name, Method: zip.Deflate, Modified: file.Modified}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, http.StatusInternalServerError
		}
		if _, err = entry.Write(data); err != nil {
			return nil, http.StatusInternalServerError
		}
	}
	if err = writer.Close(); err != nil {
		return nil, http.StatusInternalServerError
	}
	return output.Bytes(), http.StatusOK
}

func rewriteDownloadedPlugin(name string, data []byte, baseURL string) []byte {
	if name == "affilio-wp/affilio.php" {
		replacement := ""
		base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if base != "" && !strings.Contains(strings.ToLower(base), "railway.app") {
			replacement = base + "/v1"
		}
		data = bytes.ReplaceAll(data, []byte("https://affilio-production.up.railway.app/v1"), []byte(replacement))
	}
	return bytes.ReplaceAll(data, []byte("railway.app"), nil)
}

func (h *HostedHandler) placementReceiptHTML(p *database.Placement) template.HTML {
	var receipt struct {
		TrackedLink       string `json:"tracked_link"`
		PublicationStatus string `json:"publication_status"`
	}
	if json.Unmarshal(p.Receipt, &receipt) != nil || receipt.TrackedLink == "" {
		return ""
	}
	return template.HTML(fmt.Sprintf(`<section><h2>Your tracked link</h2><pre>%s</pre><p>Publication: %s. Use this link with the approved recommendation and disclosure.</p></section>`, template.HTMLEscapeString(h.BaseURL+receipt.TrackedLink), template.HTMLEscapeString(strings.ReplaceAll(receipt.PublicationStatus, "_", " "))))
}
