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

// AuthController handles HTTP endpoints for user registration, authentication, and profile.
type AuthController interface {
	Register(c fiber.Ctx) error
	Login(c fiber.Ctx) error
	GetProfile(c fiber.Ctx) error
}

type authControllerImpl struct {
	authService service.AuthService
	validator   *validator.CustomValidator
}

// NewAuthController creates a new AuthController instance.
func NewAuthController(authService service.AuthService, val *validator.CustomValidator) AuthController {
	return &authControllerImpl{
		authService: authService,
		validator:   val,
	}
}

// Register handles POST /api/v1/auth/register
func (ctrl *authControllerImpl) Register(c fiber.Ctx) error {
	var req web.RegisterRequest
	if err := request.ReadRequestBody(c, &req, ctrl.validator); err != nil {
		return err
	}

	res, err := ctrl.authService.Register(c.Context(), &req)
	if err != nil {
		return err
	}

	return response.SuccessResponse(c, http.StatusCreated, res, "User registered successfully")
}

// Login handles POST /api/v1/auth/login
func (ctrl *authControllerImpl) Login(c fiber.Ctx) error {
	var req web.LoginRequest
	if err := request.ReadRequestBody(c, &req, ctrl.validator); err != nil {
		return err
	}

	res, err := ctrl.authService.Login(c.Context(), &req)
	if err != nil {
		return err
	}

	return response.SuccessResponse(c, http.StatusOK, res, "Login successful")
}

// GetProfile handles GET /api/v1/auth/me
func (ctrl *authControllerImpl) GetProfile(c fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	userID, ok := userIDVal.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return errs.NewUnauthorizedError("unauthorized access")
	}

	res, err := ctrl.authService.GetProfile(c.Context(), userID)
	if err != nil {
		return err
	}

	return response.SuccessResponse(c, http.StatusOK, res)
}
