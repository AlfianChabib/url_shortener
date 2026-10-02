package request

import (
	"fmt"
	"url_shortener/pkg/errs"
	"url_shortener/pkg/validator"

	"github.com/gofiber/fiber/v2"
)

// ReadRequestBody parses the JSON body into target struct and validates it.
func ReadRequestBody(c *fiber.Ctx, target interface{}, customVal *validator.CustomValidator) error {
	if err := c.BodyParser(target); err != nil {
		return errs.NewBadRequestError(fmt.Sprintf("Invalid JSON request body: %v", err))
	}

	if customVal != nil {
		if err := customVal.ValidateStruct(target); err != nil {
			return err
		}
	}

	return nil
}

