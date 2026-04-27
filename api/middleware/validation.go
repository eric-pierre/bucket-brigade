package middleware

import (
	"bucket-brigade/config"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func ValidateObjectParams(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		bucket := c.Param("bucket")
		objectId := c.Param("objectId")
		maxLength := cfg.Server.Validation.ObjectRouteParamMaxLength
		if maxLength <= 0 {
			maxLength = 255
		}

		if detail := validateObjectRouteParam("bucket", bucket, maxLength); detail != "" {
			c.Error(NewBadRequestError(detail, c.Request.URL.Path, map[string]string{
				"bucket":   bucket,
				"objectId": objectId,
			}))
			c.Abort()
			return
		}

		if detail := validateObjectRouteParam("objectId", objectId, maxLength); detail != "" {
			c.Error(NewBadRequestError(detail, c.Request.URL.Path, map[string]string{
				"bucket":   bucket,
				"objectId": objectId,
			}))
			c.Abort()
			return
		}

		c.Next()
	}
}

func validateObjectRouteParam(name, value string, maxLength int) string {
	if len(value) == 0 {
		return newValidationError(name, "must not be empty")
	}
	if len(value) > maxLength {
		return newValidationError(name, "must be at most "+strconv.Itoa(maxLength)+" characters")
	}
	if strings.ContainsAny(value, "\x00\n\r") {
		return newValidationError(name, "must not contain control characters")
	}
	return ""
}

func newValidationError(name, reason string) string {
	return name + " " + reason
}
