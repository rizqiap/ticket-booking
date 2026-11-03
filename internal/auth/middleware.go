package auth

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware validates Bearer access tokens and stores identity in the request context.
func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.Fields(header)

		// Require the Authorization header to contain a Bearer token.
		if len(parts) != 2 ||
			!strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{"error": "unauthorized"},
			)

			return
		}

		var claims AccessClaims

		// Verify the signature, expected algorithm, issuer, and expiration.
		token, err := jwt.ParseWithClaims(
			parts[1],
			&claims,
			func(token *jwt.Token) (any, error) {
				return []byte(secret), nil
			},
			jwt.WithValidMethods(
				[]string{jwt.SigningMethodHS256.Alg()},
			),
			jwt.WithIssuer("ticket-booking"),
			jwt.WithExpirationRequired(),
		)

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{"error": "invalid or expired token"},
			)

			return
		}

		// The token subject must identify a user with a positive numeric ID.
		userID, err := strconv.ParseInt(
			claims.Subject, 10, 64,
		)

		if err != nil || userID <= 0 {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{"error": "invalid token subject"},
			)
			return
		}

		// Make the verified identity available to subsequent handlers.
		c.Set("user_id", userID)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// RequireRole checks the token's role after auth middleware.
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// get role from gin context
		role := c.GetString("role")

		// check permission
		if role != requiredRole {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"error": "forbidden",
				},
			)
			return
		}

		c.Next()
	}
}
