package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
		for _, name := range []string{"workspace.css", "workspace.js", "setup-controls.js", "landing.css", "landing.js", "landing-scene.bundle.js", "landing-scene.bundle.js.LEGAL.txt", "THIRD_PARTY_NOTICES.md"} {
			file := name
			public.GET("/assets/"+file, func(c *gin.Context) { c.File(filepath.Join(h.AssetsPath, file)) })
		}
		public.GET("/downloads/distributoor-setup-0.1.0.tgz", func(c *gin.Context) {
			file := filepath.Join(h.DownloadsPath, "distributoor-setup-0.1.0.tgz")
			if h.DownloadsPath == "" {
				c.Status(503)
				return
			}
			if _, err := os.Stat(file); err != nil {
				c.Status(503)
				return
			}
			c.FileAttachment(file, "distributoor-setup-0.1.0.tgz")
		})
		public.GET("/downloads/affilio.zip", h.DownloadWordPress)
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

func (h *HostedHandler) installCommand() string {
	return "npm exec --yes --package=" + h.BaseURL + "/downloads/distributoor-setup-0.1.0.tgz -- distributoor-setup --api-url " + h.BaseURL
}

func (h *HostedHandler) Home(c *gin.Context) {
	page, err := template.ParseFiles(filepath.Join(h.AssetsPath, "home.html"))
	if err != nil {
		c.Status(500)
		return
	}
	_, installerErr := os.Stat(filepath.Join(h.DownloadsPath, "distributoor-setup-0.1.0.tgz"))
	_, pluginErr := os.Stat(filepath.Join(h.DownloadsPath, "affilio.zip"))
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err = page.Execute(c.Writer, gin.H{"BaseURL": h.BaseURL, "InstallCommand": h.installCommand(), "InstallerReady": h.DownloadsPath != "" && installerErr == nil, "PluginReady": h.DownloadsPath != "" && pluginErr == nil}); err != nil {
		c.Abort()
	}
}

func (h *HostedHandler) DownloadWordPress(c *gin.Context) {
	if h.DownloadsPath == "" {
		c.Status(503)
		return
	}
	source, err := zip.OpenReader(filepath.Join(h.DownloadsPath, "affilio.zip"))
	if err != nil {
		c.Status(503)
		return
	}
	defer source.Close()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range source.File {
		if file.UncompressedSize64 > 10<<20 {
			c.Status(500)
			return
		}
		input, err := file.Open()
		if err != nil {
			c.Status(500)
			return
		}
		data, err := io.ReadAll(io.LimitReader(input, 10<<20))
		input.Close()
		if err != nil {
			c.Status(500)
			return
		}
		data = rewriteDownloadedPlugin(file.Name, data, h.BaseURL)
		header := &zip.FileHeader{Name: file.Name, Method: zip.Deflate, Modified: file.Modified}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			c.Status(500)
			return
		}
		if _, err = entry.Write(data); err != nil {
			c.Status(500)
			return
		}
	}
	if err = writer.Close(); err != nil {
		c.Status(500)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="affilio.zip"`)
	c.Data(200, "application/zip", output.Bytes())
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
