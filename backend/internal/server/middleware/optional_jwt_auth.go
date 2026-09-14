package middleware

import (
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OptionalJWTAuthMiddleware attaches an authenticated subject when a valid
// Bearer token is present, and continues anonymously otherwise.
type OptionalJWTAuthMiddleware gin.HandlerFunc

func NewOptionalJWTAuthMiddleware(authService *service.AuthService, userService *service.UserService) OptionalJWTAuthMiddleware {
	return OptionalJWTAuthMiddleware(optionalJWTAuth(authService, userService, userService))
}

func optionalJWTAuth(authService *service.AuthService, userService jwtUserReader, activityToucher userActivityToucher) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			c.Next()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Next()
			return
		}
		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" || authService == nil || userService == nil {
			c.Next()
			return
		}
		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			if errors.Is(err, service.ErrTokenExpired) {
				c.Next()
				return
			}
			c.Next()
			return
		}
		user, err := userService.GetByID(c.Request.Context(), claims.UserID)
		if err != nil || user == nil || !user.IsActive() {
			c.Next()
			return
		}
		if claims.TokenVersion != user.TokenVersion {
			c.Next()
			return
		}
		c.Set(string(ContextKeyUser), AuthSubject{
			UserID:      user.ID,
			Concurrency: user.Concurrency,
		})
		c.Set(string(ContextKeyUserRole), user.Role)
		if activityToucher != nil {
			activityToucher.TouchLastActiveForUser(c.Request.Context(), user)
		}
		c.Next()
	}
}
