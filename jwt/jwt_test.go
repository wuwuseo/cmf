package jwt_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	cmfjwt "github.com/wuwuseo/cmf/jwt"
)

const testSecret = "test-secret-key"

// ======================== CreateToken 测试 ========================

// TestCreateToken_StandardClaims 测试使用标准 RegisteredClaims 创建 token
func TestCreateToken_StandardClaims(t *testing.T) {
	claims := jwt.RegisteredClaims{
		Subject:  "user-123",
		Issuer:   "cmf",
		Audience: jwt.ClaimStrings{"api"},
	}
	tokenStr, err := cmfjwt.CreateToken(claims, testSecret)
	if err != nil {
		t.Fatalf("创建 token 失败: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("token 字符串不应为空")
	}
}

// TestCreateToken_WithExpiration 测试创建带过期时间的 token
func TestCreateToken_WithExpiration(t *testing.T) {
	expireAt := time.Now().Add(1 * time.Hour)
	claims := jwt.RegisteredClaims{
		Subject:   "user-456",
		ExpiresAt: jwt.NewNumericDate(expireAt),
	}
	tokenStr, err := cmfjwt.CreateToken(claims, testSecret)
	if err != nil {
		t.Fatalf("创建带过期时间的 token 失败: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("token 字符串不应为空")
	}

	parsedToken, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(testSecret), nil
		})
	if err != nil {
		t.Fatalf("解析 token 失败: %v", err)
	}
	parsedClaims, ok := parsedToken.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsedToken.Valid {
		t.Fatal("解析后的 token 应有效")
	}
	if parsedClaims.Subject != "user-456" {
		t.Errorf("Subject 不正确, 期望 %q, 实际 %q", "user-456", parsedClaims.Subject)
	}
	if parsedClaims.ExpiresAt == nil {
		t.Fatal("ExpiresAt 不应为 nil")
	}
}

// TestCreateToken_CustomClaims 测试使用 MapClaims 创建自定义 claims 的 token
func TestCreateToken_CustomClaims(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":      "user-789",
		"role":     "admin",
		"tenantId": "tenant-001",
		"iat":      time.Now().Unix(),
	}
	tokenStr, err := cmfjwt.CreateToken(claims, testSecret)
	if err != nil {
		t.Fatalf("创建自定义 claims token 失败: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("token 字符串不应为空")
	}

	parsedToken, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("解析 token 失败: %v", err)
	}
	parsedClaims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok || !parsedToken.Valid {
		t.Fatal("解析后的 token 应有效")
	}
	if parsedClaims["sub"] != "user-789" {
		t.Errorf("sub 不正确, 期望 %q, 实际 %v", "user-789", parsedClaims["sub"])
	}
	if parsedClaims["role"] != "admin" {
		t.Errorf("role 不正确, 期望 %q, 实际 %v", "admin", parsedClaims["role"])
	}
	if parsedClaims["tenantId"] != "tenant-001" {
		t.Errorf("tenantId 不正确, 期望 %q, 实际 %v", "tenant-001", parsedClaims["tenantId"])
	}
}

// TestCreateToken_RegisteredClaimsType 测试使用 RegisteredClaims 创建 token 并验证可 parse
func TestCreateToken_RegisteredClaimsType(t *testing.T) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		ID:        "jti-001",
		Subject:   "user-registered",
		Issuer:    "cmf-test",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
		NotBefore: jwt.NewNumericDate(now),
	}
	tokenStr, err := cmfjwt.CreateToken(claims, testSecret)
	if err != nil {
		t.Fatalf("创建 RegisteredClaims token 失败: %v", err)
	}

	parsedToken, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				t.Errorf("签名方法不正确: %v", token.Header["alg"])
			}
			return []byte(testSecret), nil
		})
	if err != nil {
		t.Fatalf("解析 token 失败: %v", err)
	}
	parsedClaims, ok := parsedToken.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsedToken.Valid {
		t.Fatal("解析后的 token 应有效")
	}
	if parsedClaims.ID != "jti-001" {
		t.Errorf("ID 不正确, 期望 %q, 实际 %q", "jti-001", parsedClaims.ID)
	}
	if parsedClaims.Subject != "user-registered" {
		t.Errorf("Subject 不正确, 期望 %q, 实际 %q", "user-registered", parsedClaims.Subject)
	}
	if parsedClaims.Issuer != "cmf-test" {
		t.Errorf("Issuer 不正确, 期望 %q, 实际 %q", "cmf-test", parsedClaims.Issuer)
	}
}

// TestCreateToken_jwtParse 测试创建的 token 能通过 jwt.Parse 解析
func TestCreateToken_jwtParse(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":  "parse-test-user",
		"name": "测试用户",
	}
	tokenStr, err := cmfjwt.CreateToken(claims, testSecret)
	if err != nil {
		t.Fatalf("创建 token 失败: %v", err)
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("jwt.Parse 解析 token 失败: %v", err)
	}
	if !token.Valid {
		t.Fatal("token 应有效")
	}
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("claims 应为 MapClaims 类型")
	}
	if mapClaims["sub"] != "parse-test-user" {
		t.Errorf("sub 不正确, 期望 %q, 实际 %v", "parse-test-user", mapClaims["sub"])
	}
	if mapClaims["name"] != "测试用户" {
		t.Errorf("name 不正确, 期望 %q, 实际 %v", "测试用户", mapClaims["name"])
	}
}

// TestCreateToken_EmptySecret 测试使用空 secret 创建 token
func TestCreateToken_EmptySecret(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": "user-empty-secret",
	}
	tokenStr, err := cmfjwt.CreateToken(claims, "")
	if err != nil {
		t.Fatalf("使用空 secret 创建 token 失败: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("token 字符串不应为空")
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(""), nil
	})
	if err != nil {
		t.Fatalf("使用空 secret 解析 token 失败: %v", err)
	}
	if !token.Valid {
		t.Fatal("token 应有效")
	}
}

// TestCreateToken_DifferentSecretFails 测试使用不同 secret 验证 token 失败
func TestCreateToken_DifferentSecretFails(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": "user-secret-test",
	}
	tokenStr, err := cmfjwt.CreateToken(claims, testSecret)
	if err != nil {
		t.Fatalf("创建 token 失败: %v", err)
	}

	_, err = jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte("wrong-secret-key"), nil
	})
	if err == nil {
		t.Fatal("使用错误 secret 解析 token 应该失败")
	}
}

// ======================== GetJWTUserData 测试 ========================

// TestGetJWTUserData_Success 测试在有效 token 的情况下能正确获取用户数据
func TestGetJWTUserData_Success(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":      "user-data-test",
		"username": "testuser",
		"role":     "member",
	}
	tokenStr, err := cmfjwt.CreateToken(claims, testSecret)
	if err != nil {
		t.Fatalf("创建 token 失败: %v", err)
	}

	app := fiber.New()
	middleware := cmfjwt.NewJWTMiddleware(testSecret)
	app.Use(middleware)

	var capturedClaims jwt.MapClaims
	app.Get("/test", func(c fiber.Ctx) error {
		capturedClaims = cmfjwt.GetJWTUserData(c)
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("发送测试请求失败: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("期望状态码 200, 实际 %d", resp.StatusCode)
	}
	if capturedClaims == nil {
		t.Fatal("capturedClaims 不应为 nil")
	}
	if capturedClaims["sub"] != "user-data-test" {
		t.Errorf("sub 不正确, 期望 %q, 实际 %v", "user-data-test", capturedClaims["sub"])
	}
	if capturedClaims["username"] != "testuser" {
		t.Errorf("username 不正确, 期望 %q, 实际 %v", "testuser", capturedClaims["username"])
	}
	if capturedClaims["role"] != "member" {
		t.Errorf("role 不正确, 期望 %q, 实际 %v", "member", capturedClaims["role"])
	}
}

// TestGetJWTUserData_NoToken 测试请求中无 token 时返回 nil
func TestGetJWTUserData_NoToken(t *testing.T) {
	app := fiber.New()

	var capturedClaims jwt.MapClaims
	app.Get("/no-auth", func(c fiber.Ctx) error {
		capturedClaims = cmfjwt.GetJWTUserData(c)
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/no-auth", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("发送测试请求失败: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("期望状态码 200, 实际 %d", resp.StatusCode)
	}
	if capturedClaims != nil {
		t.Error("无 token 时 capturedClaims 应为 nil")
	}
}

// TestGetJWTUserData_InvalidToken 测试使用无效 token 时的行为
func TestGetJWTUserData_InvalidToken(t *testing.T) {
	app := fiber.New()
	middleware := cmfjwt.NewJWTMiddleware(testSecret)
	app.Use(middleware)

	var handlerCalled bool
	app.Get("/test", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")

	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("发送测试请求失败: %v", err)
	}
	if handlerCalled {
		t.Error("使用无效 token 时 handler 不应被调用")
	}
}

// ======================== NewJWTMiddleware 测试 ========================

// TestNewJWTMiddleware_NotNil 测试创建默认中间件不为 nil
func TestNewJWTMiddleware_NotNil(t *testing.T) {
	middleware := cmfjwt.NewJWTMiddleware(testSecret)
	if middleware == nil {
		t.Fatal("NewJWTMiddleware 返回的中间件不应为 nil")
	}
}

// TestNewJWTMiddlewareWithConfig_SuccessHandler 测试自定义 SuccessHandler
func TestNewJWTMiddlewareWithConfig_SuccessHandler(t *testing.T) {
	successCalled := false

	cfg := cmfjwt.JWTConfig{
		Secret: testSecret,
		SuccessHandler: func(c fiber.Ctx) error {
			successCalled = true
			return c.Next()
		},
	}

	claims := jwt.MapClaims{
		"sub": "success-handler-user",
	}
	tokenStr, err := cmfjwt.CreateToken(claims, testSecret)
	if err != nil {
		t.Fatalf("创建 token 失败: %v", err)
	}

	app := fiber.New()
	app.Use(cmfjwt.NewJWTMiddlewareWithConfig(cfg))

	var handlerCalled bool
	app.Get("/test", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("发送测试请求失败: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("期望状态码 200, 实际 %d", resp.StatusCode)
	}
	if !successCalled {
		t.Error("SuccessHandler 应被调用")
	}
	if !handlerCalled {
		t.Error("实际的 handler 应被调用")
	}
}

// TestNewJWTMiddlewareWithConfig_ErrorHandler 测试自定义 ErrorHandler
func TestNewJWTMiddlewareWithConfig_ErrorHandler(t *testing.T) {
	errorCalled := false

	cfg := cmfjwt.JWTConfig{
		Secret: testSecret,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			errorCalled = true
			return c.Status(fiber.StatusUnauthorized).SendString("自定义错误: " + err.Error())
		},
	}

	app := fiber.New()
	app.Use(cmfjwt.NewJWTMiddlewareWithConfig(cfg))

	var handlerCalled bool
	app.Get("/test", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("发送测试请求失败: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("期望状态码 401, 实际 %d", resp.StatusCode)
	}
	if !errorCalled {
		t.Error("ErrorHandler 应被调用")
	}
	if handlerCalled {
		t.Error("使用无效 token 时 handler 不应被调用")
	}
}

// ======================== 集成测试 ========================

// TestFullFlow_CreateTokenAndGetUserData 测试完整的创建 token 并在中间件中获取用户数据的流程
func TestFullFlow_CreateTokenAndGetUserData(t *testing.T) {
	secret := "integration-secret"
	claims := jwt.MapClaims{
		"sub":      "integration-user",
		"username": "integration-test",
		"role":     "admin",
		"email":    "test@example.com",
	}

	tokenStr, err := cmfjwt.CreateToken(claims, secret)
	if err != nil {
		t.Fatalf("创建 token 失败: %v", err)
	}

	app := fiber.New()
	app.Use(cmfjwt.NewJWTMiddleware(secret))

	var userData jwt.MapClaims
	app.Get("/user", func(c fiber.Ctx) error {
		userData = cmfjwt.GetJWTUserData(c)
		if userData == nil {
			return c.Status(fiber.StatusInternalServerError).SendString("no user data")
		}
		return c.JSON(userData)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/user", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("发送测试请求失败: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("期望状态码 200, 实际 %d", resp.StatusCode)
	}
	if userData == nil {
		t.Fatal("userData 不应为 nil")
	}
	if userData["sub"] != "integration-user" {
		t.Errorf("sub 不正确, 期望 %q, 实际 %v", "integration-user", userData["sub"])
	}
	if userData["username"] != "integration-test" {
		t.Errorf("username 不正确, 期望 %q, 实际 %v", "integration-test", userData["username"])
	}
	if userData["role"] != "admin" {
		t.Errorf("role 不正确, 期望 %q, 实际 %v", "admin", userData["role"])
	}
	if userData["email"] != "test@example.com" {
		t.Errorf("email 不正确, 期望 %q, 实际 %v", "test@example.com", userData["email"])
	}
}
