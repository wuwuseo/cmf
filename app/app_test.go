package app

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func testApp() App {
	return App{
		Code: "demo", Name: "Demo", Enabled: true,
		Routes: []Route{{Method: "GET", Path: "/items/:id", Auth: AuthAdmin, Handler: func(c fiber.Ctx) error {
			return c.SendString("ok")
		}}},
		Permissions: []Permission{
			{Type: PermMenu, Code: "demo:menu", Name: "Menu", MenuPath: "/demo"},
			{Type: PermAPI, Code: "demo:item", Name: "Item", Path: "/api/v1/demo/items/{id}", Method: "GET", Parent: "demo:menu"},
		},
	}
}

func TestValidateApps(t *testing.T) {
	opts := Options{BasePath: "/api/v1", ReservedCodes: []string{"admin"}}
	for _, tc := range []struct {
		name string
		edit func(*App)
		want string
	}{
		{"valid", func(*App) {}, ""},
		{"reserved code", func(a *App) { a.Code = "admin" }, "app code"},
		{"missing API permission", func(a *App) { a.Permissions = a.Permissions[:1] }, "no API permission"},
		{"foreign API permission", func(a *App) { a.Permissions[1].Path = "/api/v1/admin/users" }, "invalid API permission"},
		{"orphan API permission", func(a *App) { a.Routes = nil }, "no admin route"},
		{"missing parent", func(a *App) { a.Permissions[1].Parent = "demo:missing" }, "missing parent"},
		{"parent cycle", func(a *App) { a.Permissions[0].Parent = "demo:item" }, "parent cycle"},
		{"duplicate route", func(a *App) { a.Routes = append(a.Routes, a.Routes[0]) }, "duplicate route"},
		{"equivalent parameters", func(a *App) {
			a.Routes = append(a.Routes, Route{Method: "GET", Path: "/items/:other", Auth: AuthAdmin, Handler: a.Routes[0].Handler})
		}, "duplicate route"},
		{"overlapping auth", func(a *App) {
			a.Routes = append(a.Routes, Route{Method: "GET", Path: "/items/public", Auth: AuthPublic, Handler: a.Routes[0].Handler})
		}, "overlapping routes"},
		{"disabled skips declarations", func(a *App) { a.Enabled = false; a.Routes[0].Handler = nil }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			tc.edit(&a)
			err := ValidateApps([]App{a}, opts)
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("ValidateApps() = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestMountMiddlewareOrder(t *testing.T) {
	a := testApp()
	a.Code = "mountdemo"
	a.Permissions[0].Code = "mountdemo:menu"
	a.Permissions[1].Code = "mountdemo:item"
	a.Permissions[1].Parent = "mountdemo:menu"
	a.Permissions[1].Path = "/api/v1/mountdemo/items/{id}"
	Register(a)
	f := fiber.New()
	v1 := f.Group("/api/v1")
	var order []string
	err := Mount(v1, Options{BasePath: "/api/v1", Middleware: func(Auth) []fiber.Handler {
		return []fiber.Handler{func(c fiber.Ctx) error {
			order = append(order, "middleware")
			return c.Next()
		}}
	}})
	if err != nil {
		t.Fatal(err)
	}
	response, err := f.Test(httptest.NewRequest("GET", "/api/v1/mountdemo/items/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || len(order) != 1 || order[0] != "middleware" {
		t.Fatalf("status=%d order=%v", response.StatusCode, order)
	}
}
