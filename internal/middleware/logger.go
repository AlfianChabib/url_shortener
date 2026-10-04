package middleware

import (
	"io"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

// NewLoggerMiddleware creates an HTTP request logger middleware.
func NewLoggerMiddleware(output ...io.Writer) fiber.Handler {
	var out io.Writer = os.Stdout
	if len(output) > 0 && output[0] != nil {
		out = output[0]
	}

	return logger.New(logger.Config{
		Stream:     out,
		Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	})
}
