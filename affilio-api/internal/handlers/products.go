package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type ProductsHandler struct {
	DB *database.DB
}

type productPayload struct {
	Name            *string  `json:"name"`
	Tagline         *string  `json:"tagline"`
	WebsiteURL      *string  `json:"website_url"`
	DocsURL         *string  `json:"docs_url"`
	PricingURL      *string  `json:"pricing_url"`
	ProductType     *string  `json:"product_type"`
	PaymentProvider *string  `json:"payment_provider"`
	PrimaryCategory *string  `json:"primary_category"`
	Tags            []string `json:"tags"`
	Keywords        []string `json:"keywords"`
}

var validProductTypes = map[string]bool{
	"saas": true, "game": true, "shopify_app": true, "dev_tool": true, "other": true,
}

var validPaymentProviders = map[string]bool{
	"stripe": true, "lemonsqueezy": true, "polar": true, "none": true, "": true,
}

func makerID(c *gin.Context) string {
	return c.GetString("maker_id")
}

func (h *ProductsHandler) Create(c *gin.Context) {
	var req productPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	p := &database.Product{MakerID: makerID(c), ProductType: "other"}
	if err := applyProductPayload(p, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if p.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if p.WebsiteURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "website_url is required"})
		return
	}
	id, err := h.DB.CreateProduct(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create product"})
		return
	}
	created, err := h.DB.GetProduct(c.Request.Context(), p.MakerID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}
	c.JSON(http.StatusOK, productJSON(created))
}

func (h *ProductsHandler) List(c *gin.Context) {
	products, err := h.DB.ListProducts(c.Request.Context(), makerID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load products"})
		return
	}
	out := make([]gin.H, 0, len(products))
	for i := range products {
		out = append(out, productJSON(&products[i]))
	}
	c.JSON(http.StatusOK, gin.H{"products": out})
}

func (h *ProductsHandler) Get(c *gin.Context) {
	product, err := h.DB.GetProduct(c.Request.Context(), makerID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}
	c.JSON(http.StatusOK, productJSON(product))
}

func (h *ProductsHandler) Patch(c *gin.Context) {
	product, err := h.DB.GetProduct(c.Request.Context(), makerID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}
	var req productPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := applyProductPayload(product, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if product.Name == "" || product.WebsiteURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and website_url cannot be empty"})
		return
	}
	if err := h.DB.UpdateProduct(c.Request.Context(), product); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update product"})
		return
	}
	updated, err := h.DB.GetProduct(c.Request.Context(), product.MakerID, product.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}
	c.JSON(http.StatusOK, productJSON(updated))
}

func applyProductPayload(p *database.Product, req *productPayload) error {
	if req.Name != nil {
		p.Name = strings.TrimSpace(*req.Name)
		if len(p.Name) > 255 {
			return errors.New("name too long")
		}
	}
	if req.Tagline != nil {
		p.Tagline = strings.TrimSpace(*req.Tagline)
		if len(p.Tagline) > 512 {
			return errors.New("tagline too long")
		}
	}
	if req.WebsiteURL != nil {
		u, err := normalizeHTTPSURL(*req.WebsiteURL)
		if err != nil {
			return errors.New("website_url must be an https URL")
		}
		p.WebsiteURL = u
	}
	if req.DocsURL != nil {
		u, err := normalizeOptionalHTTPURL(*req.DocsURL)
		if err != nil {
			return errors.New("docs_url must be a valid http(s) url")
		}
		p.DocsURL = u
	}
	if req.PricingURL != nil {
		u, err := normalizeOptionalHTTPURL(*req.PricingURL)
		if err != nil {
			return errors.New("pricing_url must be a valid http(s) url")
		}
		p.PricingURL = u
	}
	if req.ProductType != nil {
		t := strings.TrimSpace(strings.ToLower(*req.ProductType))
		if !validProductTypes[t] {
			return errors.New("invalid product_type")
		}
		p.ProductType = t
	}
	if req.PaymentProvider != nil {
		provider := strings.TrimSpace(strings.ToLower(*req.PaymentProvider))
		if !validPaymentProviders[provider] {
			return errors.New("invalid payment_provider")
		}
		p.PaymentProvider = provider
	}
	if req.PrimaryCategory != nil {
		p.PrimaryCategory = strings.TrimSpace(strings.ToLower(*req.PrimaryCategory))
	}
	if req.Tags != nil {
		p.Tags = trimStrings(req.Tags)
	}
	if req.Keywords != nil {
		p.Keywords = trimStrings(req.Keywords)
	}
	return nil
}

func productJSON(p *database.Product) gin.H {
	return gin.H{
		"id":               p.ID,
		"name":             p.Name,
		"tagline":          p.Tagline,
		"website_url":      p.WebsiteURL,
		"docs_url":         p.DocsURL,
		"pricing_url":      p.PricingURL,
		"product_type":     p.ProductType,
		"payment_provider": p.PaymentProvider,
		"primary_category": p.PrimaryCategory,
		"tags":             p.Tags,
		"keywords":         p.Keywords,
		"readiness_score":  p.ReadinessScore,
		"readiness_status": p.ReadinessStatus,
		"created_at":       p.CreatedAt,
		"updated_at":       p.UpdatedAt,
	}
}

func normalizeHTTPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("invalid url")
	}
	if len(raw) > 512 {
		return "", errors.New("url too long")
	}
	return raw, nil
}

func normalizeHTTPSURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", errors.New("invalid url")
	}
	if len(raw) > 512 {
		return "", errors.New("url too long")
	}
	return raw, nil
}

func normalizeOptionalHTTPURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return normalizeHTTPURL(raw)
}

func trimStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
