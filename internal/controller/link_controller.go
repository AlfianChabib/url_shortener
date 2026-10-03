package controller

import (
	"net/http"
	"url_shortener/internal/helper/request"
	"url_shortener/internal/helper/response"
	"url_shortener/internal/model/web"
	"url_shortener/internal/service"
	"url_shortener/pkg/errs"
	"url_shortener/pkg/validator"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// LinkController handles link creation, redirection, and analytics.
type LinkController interface {
	Create(c fiber.Ctx) error
	Redirect(c fiber.Ctx) error
	GetAnalytics(c fiber.Ctx) error
}

type linkControllerImpl struct {
	service   service.LinkService
	validator *validator.CustomValidator
}

// NewLinkController creates a new LinkController instance.
func NewLinkController(service service.LinkService, val *validator.CustomValidator) LinkController {
	return &linkControllerImpl{
		service:   service,
		validator: val,
	}
}

// Create handles POST /api/v1/links
func (ctrl *linkControllerImpl) Create(c fiber.Ctx) error {
	var req web.CreateLinkRequest
	if err := request.ReadRequestBody(c, &req, ctrl.validator); err != nil {
		return err
	}

	var userID *uuid.UUID
	if val := c.Locals("user_id"); val != nil {
		if uid, ok := val.(uuid.UUID); ok && uid != uuid.Nil {
			userID = &uid
		}
	}

	res, err := ctrl.service.CreateShortLink(c.Context(), &req, userID)
	if err != nil {
		return err
	}

	return response.SuccessResponse(c, http.StatusCreated, res, "Link shortened successfully")
}

// Redirect handles GET /{short_code}
func (ctrl *linkControllerImpl) Redirect(c fiber.Ctx) error {
	shortCode := c.Params("short_code")
	if shortCode == "" {
		return errs.NewBadRequestError("short code is required")
	}

	originalURL, err := ctrl.service.GetOriginalURL(c.Context(), shortCode)
	if err != nil {
		return err
	}

	// PRD Section 3.2: Use HTTP 307 Temporary Redirect to prevent client-side permanent caching
	c.Set("Cache-Control", "private, max-age=60")
	return c.Redirect().Status(http.StatusTemporaryRedirect).To(originalURL)
}

// GetAnalytics handles GET /api/v1/links/:short_code/analytics
func (ctrl *linkControllerImpl) GetAnalytics(c fiber.Ctx) error {
	shortCode := c.Params("short_code")
	if shortCode == "" {
		return errs.NewBadRequestError("short code is required")
	}

	res, err := ctrl.service.GetAnalytics(c.Context(), shortCode)
	if err != nil {
		return err
	}

	return response.SuccessResponse(c, http.StatusOK, res)
}
