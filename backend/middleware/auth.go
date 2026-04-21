package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/pkg/resp"
)

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

const claimsKey = "claims"

func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ""
		header := c.GetHeader("Authorization")
		if strings.HasPrefix(header, "Bearer ") {
			tokenStr = strings.TrimPrefix(header, "Bearer ")
		} else {
			// Fallback: accept ?token= for WebSocket upgrades and file downloads.
			tokenStr = c.Query("token")
		}
		if tokenStr == "" {
			resp.Unauthorized(c, "missing token")
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.Security.JWTSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
		if err != nil || !token.Valid {
			resp.Unauthorized(c, "invalid token")
			return
		}
		// Tokens minted for the TOTP second-step exchange are not full session
		// tokens and must never authorize protected routes.
		if claims.Role == "tmp_totp" {
			resp.Unauthorized(c, "token not valid for this route")
			return
		}

		c.Set(claimsKey, claims)
		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func GetClaims(c *gin.Context) *Claims {
	v, _ := c.Get(claimsKey)
	claims, _ := v.(*Claims)
	return claims
}

func ParseToken(tokenStr, secret string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}
