package http_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	cmfhttp "github.com/wuwuseo/cmf/http"
)

// ======================== NewApiResponse 创建实例 ========================

// TestNewApiResponse 测试 NewApiResponse 能正确创建实例
func TestNewApiResponse(t *testing.T) {
	app := fiber.New()

	app.Get("/new", func(c fiber.Ctx) error {
		resp := cmfhttp.NewApiResponse(c)
		if resp == nil {
			t.Fatal("NewApiResponse 应返回非 nil 的实例")
		}
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/new", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际: %d", resp.StatusCode)
	}
}

// ======================== ToResponse 设置状态码和 JSON ========================

// TestToResponse 测试 ToResponse 能正确设置状态码和 JSON 响应体
func TestToResponse(t *testing.T) {
	app := fiber.New()

	app.Get("/to-response", func(c fiber.Ctx) error {
		resp := cmfhttp.NewApiResponse(c)
		return resp.ToResponse(fiber.Map{
			"code": 1,
			"msg":  "自定义响应",
		}, 201)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/to-response", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("期望状态码 201，实际: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	if code, ok := body["code"].(float64); !ok || code != 1 {
		t.Errorf("期望 code=1，实际: %v", body["code"])
	}
	if msg, ok := body["msg"].(string); !ok || msg != "自定义响应" {
		t.Errorf("期望 msg='自定义响应'，实际: %v", body["msg"])
	}
}

// TestToResponse_NilData 测试当 data 为 nil 时自动初始化为空 Map
func TestToResponse_NilData(t *testing.T) {
	app := fiber.New()

	app.Get("/nil-data", func(c fiber.Ctx) error {
		resp := cmfhttp.NewApiResponse(c)
		return resp.ToResponse(nil, 200)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/nil-data", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	// nil data 传递后应被序列化为空 JSON 对象 "{}"
	if string(bodyBytes) != "{}" {
		t.Errorf("nil data 应序列化为 {}，实际: %s", string(bodyBytes))
	}
}

// ======================== Result 方法 ========================

// TestResult 测试 Result 方法返回统一格式 {code, msg, data}
func TestResult(t *testing.T) {
	app := fiber.New()

	app.Get("/result", func(c fiber.Ctx) error {
		resp := cmfhttp.NewApiResponse(c)
		return resp.Result("操作成功", fiber.Map{"id": 1, "name": "test"}, 1)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/result", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	// 验证顶层字段
	if code, ok := body["code"].(float64); !ok || code != 1 {
		t.Errorf("期望 code=1，实际: %v", body["code"])
	}
	if msg, ok := body["msg"].(string); !ok || msg != "操作成功" {
		t.Errorf("期望 msg='操作成功'，实际: %v", body["msg"])
	}

	// 验证 data 嵌套字段
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data 字段类型不正确，应为对象")
	}
	if id, ok := data["id"].(float64); !ok || id != 1 {
		t.Errorf("期望 data.id=1，实际: %v", data["id"])
	}
	if name, ok := data["name"].(string); !ok || name != "test" {
		t.Errorf("期望 data.name='test'，实际: %v", data["name"])
	}
}

// ======================== Success 方法（code=1） ========================

// TestSuccess 测试 Success 方法返回 code=1 的成功响应
func TestSuccess(t *testing.T) {
	app := fiber.New()

	app.Get("/success", func(c fiber.Ctx) error {
		resp := cmfhttp.NewApiResponse(c)
		return resp.Success("请求成功", fiber.Map{"user_id": 100})
	})

	req := httptest.NewRequest(fiber.MethodGet, "/success", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	if code, ok := body["code"].(float64); !ok || code != 1 {
		t.Errorf("Success 方法期望 code=1，实际: %v", body["code"])
	}
	if msg, ok := body["msg"].(string); !ok || msg != "请求成功" {
		t.Errorf("期望 msg='请求成功'，实际: %v", body["msg"])
	}

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data 字段类型不正确，应为对象")
	}
	if userId, ok := data["user_id"].(float64); !ok || userId != 100 {
		t.Errorf("期望 data.user_id=100，实际: %v", data["user_id"])
	}
}

// ======================== Error 方法（code=0） ========================

// TestError 测试 Error 方法返回 code=0 的错误响应
func TestError(t *testing.T) {
	app := fiber.New()

	app.Get("/error", func(c fiber.Ctx) error {
		resp := cmfhttp.NewApiResponse(c)
		return resp.Error("参数错误", fiber.Map{"field": "email"})
	})

	req := httptest.NewRequest(fiber.MethodGet, "/error", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	if code, ok := body["code"].(float64); !ok || code != 0 {
		t.Errorf("Error 方法期望 code=0，实际: %v", body["code"])
	}
	if msg, ok := body["msg"].(string); !ok || msg != "参数错误" {
		t.Errorf("期望 msg='参数错误'，实际: %v", body["msg"])
	}

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data 字段类型不正确，应为对象")
	}
	if field, ok := data["field"].(string); !ok || field != "email" {
		t.Errorf("期望 data.field='email'，实际: %v", data["field"])
	}
}

// ======================== 验证响应 JSON 格式 ========================

// TestResponseJSONFormat 验证响应的 JSON 格式完整正确
func TestResponseJSONFormat(t *testing.T) {
	app := fiber.New()

	// 注册多个路由以覆盖不同场景
	app.Get("/format-success", func(c fiber.Ctx) error {
		resp := cmfhttp.NewApiResponse(c)
		return resp.Success("OK", fiber.Map{"result": true})
	})
	app.Get("/format-error", func(c fiber.Ctx) error {
		resp := cmfhttp.NewApiResponse(c)
		return resp.Error("FAIL", nil)
	})

	tests := []struct {
		name         string
		path         string
		expectedCode float64
		expectedMsg  string
		expectedKeys []string
	}{
		{
			name:         "成功响应格式",
			path:         "/format-success",
			expectedCode: 1,
			expectedMsg:  "OK",
			expectedKeys: []string{"code", "msg", "data"},
		},
		{
			name:         "错误响应格式",
			path:         "/format-error",
			expectedCode: 0,
			expectedMsg:  "FAIL",
			expectedKeys: []string{"code", "msg", "data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(fiber.MethodGet, tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("请求失败: %v", err)
			}
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("读取响应体失败: %v", err)
			}

			var body map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Fatalf("解析响应 JSON 失败: %v", err)
			}

			// 验证必须的字段都存在
			for _, key := range tt.expectedKeys {
				if _, exists := body[key]; !exists {
					t.Errorf("响应 JSON 缺少字段: %s", key)
				}
			}

			// 验证字段值
			if code, ok := body["code"].(float64); !ok || code != tt.expectedCode {
				t.Errorf("期望 code=%v，实际: %v", tt.expectedCode, body["code"])
			}
			if msg, ok := body["msg"].(string); !ok || msg != tt.expectedMsg {
				t.Errorf("期望 msg='%s'，实际: %v", tt.expectedMsg, body["msg"])
			}
		})
	}
}
