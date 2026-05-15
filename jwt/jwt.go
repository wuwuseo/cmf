package jwt

import (
	jwtware "github.com/gofiber/contrib/v3/jwt"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig JWT 中间件配置
type JWTConfig struct {
	Secret         string            // JWT 签名密钥
	SuccessHandler fiber.Handler     // Token 校验成功后的处理函数（可选）
	ErrorHandler   fiber.ErrorHandler // Token 校验失败后的处理函数（可选）
}

// NewJWTMiddleware 使用默认配置创建 JWT 中间件
func NewJWTMiddleware(secret string) fiber.Handler {
	return NewJWTMiddlewareWithConfig(JWTConfig{Secret: secret})
}

// NewJWTMiddlewareWithConfig 使用自定义配置创建 JWT 中间件
// 支持在 SuccessHandler 中注入黑名单/退出校验、审计日志等自定义逻辑
func NewJWTMiddlewareWithConfig(cfg JWTConfig) fiber.Handler {
	jwtCfg := jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: []byte(cfg.Secret)},
	}
	if cfg.SuccessHandler != nil {
		jwtCfg.SuccessHandler = cfg.SuccessHandler
	}
	if cfg.ErrorHandler != nil {
		jwtCfg.ErrorHandler = cfg.ErrorHandler
	}
	return jwtware.New(jwtCfg)
}

func CreateToken(claims jwt.Claims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func GetJWTUserData(c fiber.Ctx) jwt.MapClaims {
	user := jwtware.FromContext(c)
	if user == nil {
		return nil
	}
	claims, _ := user.Claims.(jwt.MapClaims)
	return claims
}