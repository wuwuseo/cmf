package app

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// Validate checks all registered feature sets without changing router or storage state.
func Validate(opts Options) error { return ValidateApps(Registered(), opts) }

// ValidateApps is also useful for validating declarations before registration.
func ValidateApps(apps []App, opts Options) error {
	reserved := make(map[string]bool, len(opts.ReservedCodes))
	for _, code := range opts.ReservedCodes {
		reserved[code] = true
	}
	seenApps := make(map[string]bool, len(apps))
	seenPermissions := make(map[string]bool)
	for _, a := range apps {
		if !codePattern.MatchString(a.Code) || reserved[a.Code] || seenApps[a.Code] {
			return fmt.Errorf("invalid, reserved, or duplicate app code %q", a.Code)
		}
		seenApps[a.Code] = true
		if !a.Enabled {
			continue
		}
		if a.Name == "" {
			return fmt.Errorf("app %s: name is required", a.Code)
		}
		configKeys := map[string]bool{}
		for _, item := range a.Config {
			if item.Key == "" || item.Name == "" || item.Type == "" || strings.ContainsAny(item.Key, " /\\") || configKeys[item.Key] {
				return fmt.Errorf("app %s: invalid or duplicate config key %q", a.Code, item.Key)
			}
			configKeys[item.Key] = true
		}
		perms := make(map[string]Permission, len(a.Permissions))
		apiPermissions := map[string]bool{}
		appPath := path.Join(opts.BasePath, a.Code)
		for _, p := range a.Permissions {
			if p.Name == "" || !strings.HasPrefix(p.Code, a.Code+":") || len(p.Code) == len(a.Code)+1 || seenPermissions[p.Code] {
				return fmt.Errorf("app %s: invalid or duplicate permission code %q", a.Code, p.Code)
			}
			seenPermissions[p.Code] = true
			perms[p.Code] = p
			switch p.Type {
			case PermAPI:
				if !validAbsolutePath(p.Path) || !validMethod(p.Method) || !strings.HasPrefix(p.Path, appPath+"/") {
					return fmt.Errorf("app %s: invalid API permission %q", a.Code, p.Code)
				}
				key := p.Method + " " + p.Path
				if apiPermissions[key] {
					return fmt.Errorf("app %s: duplicate API permission %s", a.Code, key)
				}
				apiPermissions[key] = true
			case PermMenu:
				if !validAbsolutePath(p.MenuPath) {
					return fmt.Errorf("app %s: invalid menu path for %q", a.Code, p.Code)
				}
			case PermButton:
			default:
				return fmt.Errorf("app %s: invalid permission type for %q", a.Code, p.Code)
			}
		}
		state := map[string]uint8{}
		var visit func(string) error
		visit = func(code string) error {
			if state[code] == 1 {
				return fmt.Errorf("app %s: permission parent cycle at %q", a.Code, code)
			}
			if state[code] == 2 {
				return nil
			}
			state[code] = 1
			if parent := perms[code].Parent; parent != "" {
				if _, ok := perms[parent]; !ok {
					return fmt.Errorf("app %s: missing parent %q", a.Code, parent)
				}
				if err := visit(parent); err != nil {
					return err
				}
			}
			state[code] = 2
			return nil
		}
		for code := range perms {
			if err := visit(code); err != nil {
				return err
			}
		}
		var routes []Route
		adminRoutes := map[string]bool{}
		for _, r := range a.Routes {
			if !validMethod(r.Method) || !validRoutePath(r.Path) || r.Handler == nil {
				return fmt.Errorf("app %s: invalid route %q %q", a.Code, r.Method, r.Path)
			}
			if r.Auth != AuthPublic && r.Auth != AuthUser && r.Auth != AuthAdmin {
				return fmt.Errorf("app %s: invalid route auth %q", a.Code, r.Auth)
			}
			for _, previous := range routes {
				if previous.Method != r.Method {
					continue
				}
				if routeShape(previous.Path) == routeShape(r.Path) {
					return fmt.Errorf("app %s: duplicate route %s %s", a.Code, r.Method, r.Path)
				}
				if previous.Auth != r.Auth && routePathsOverlap(previous.Path, r.Path) {
					return fmt.Errorf("app %s: overlapping routes with different auth: %s and %s", a.Code, previous.Path, r.Path)
				}
			}
			routes = append(routes, r)
			if r.Auth == AuthAdmin {
				fullPath := normalizeRoutePath(path.Join(opts.BasePath, a.Code, r.Path))
				apiKey := r.Method + " " + fullPath
				if !apiPermissions[apiKey] {
					return fmt.Errorf("app %s: admin route %s %s has no API permission", a.Code, r.Method, fullPath)
				}
				adminRoutes[apiKey] = true
			}
		}
		for key := range apiPermissions {
			if !adminRoutes[key] {
				return fmt.Errorf("app %s: API permission %s has no admin route", a.Code, key)
			}
		}
	}
	return nil
}

func validMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}

func validAbsolutePath(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//") && !strings.Contains(p, "..") && path.Clean(p) == p
}

func validRoutePath(p string) bool { return validAbsolutePath(p) && !strings.Contains(p, "*") }

func routeShape(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = ":"
		}
	}
	return strings.Join(parts, "/")
}

func routePathsOverlap(a, b string) bool {
	left, right := strings.Split(a, "/"), strings.Split(b, "/")
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] && !strings.HasPrefix(left[i], ":") && !strings.HasPrefix(right[i], ":") {
			return false
		}
	}
	return true
}

func normalizeRoutePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + part[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// Mount registers enabled feature routes below router after validation.
func Mount(router fiber.Router, opts Options) error {
	if err := Validate(opts); err != nil {
		return err
	}
	for _, a := range Registered() {
		if !a.Enabled {
			continue
		}
		group := router.Group("/" + a.Code)
		for _, route := range a.Routes {
			var handlers []any
			if opts.Middleware != nil {
				for _, handler := range opts.Middleware(route.Auth) {
					if handler == nil {
						return fmt.Errorf("app %s: nil middleware for %s", a.Code, route.Auth)
					}
					handlers = append(handlers, handler)
				}
			}
			if route.Auth != AuthPublic && len(handlers) == 0 {
				return fmt.Errorf("app %s: missing middleware for %s", a.Code, route.Auth)
			}
			handlers = append(handlers, route.Handler)
			group.Add([]string{route.Method}, route.Path, handlers[0], handlers[1:]...)
		}
	}
	return nil
}
