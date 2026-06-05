package casbin

import (
	"strings"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/persist"
	"github.com/gofiber/fiber/v3"
)

const (
	MatchAllRule ValidationRule = iota
	AtLeastOneRule
)

type ValidationRule int

type PermissionParserFunc func(str string) []string

type OptionFunc func(*Options)

type Option interface {
	apply(*Options)
}

type Options struct {
	ValidationRule   ValidationRule
	PermissionParser PermissionParserFunc
}

type MiddlewareConfig struct {
	ModelFilePath string
	PolicyAdapter persist.Adapter
	Enforcer      *casbin.Enforcer
	Lookup        func(fiber.Ctx) string
	Unauthorized  fiber.Handler
	Forbidden     fiber.Handler
}

type Middleware struct {
	config MiddlewareConfig
}

var OptionsDefault = Options{
	ValidationRule:   MatchAllRule,
	PermissionParser: PermissionParserWithSeperator(":"),
}

func (of OptionFunc) apply(o *Options) {
	of(o)
}

func WithValidationRule(vr ValidationRule) Option {
	return OptionFunc(func(o *Options) {
		o.ValidationRule = vr
	})
}

func WithPermissionParser(pp PermissionParserFunc) Option {
	return OptionFunc(func(o *Options) {
		o.PermissionParser = pp
	})
}

func PermissionParserWithSeperator(sep string) PermissionParserFunc {
	return func(str string) []string {
		return strings.Split(str, sep)
	}
}

func NewCasbinMiddleware(adapter persist.Adapter, path string) *Middleware {
	enforcer, err := casbin.NewEnforcer(path, adapter)
	if err != nil {
		panic(err)
	}
	return NewMiddleware(MiddlewareConfig{
		Enforcer: enforcer,
	})
}

func NewMiddleware(config MiddlewareConfig) *Middleware {
	if config.Lookup == nil {
		config.Lookup = func(c fiber.Ctx) string { return "" }
	}
	if config.Unauthorized == nil {
		config.Unauthorized = func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusUnauthorized) }
	}
	if config.Forbidden == nil {
		config.Forbidden = func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusForbidden) }
	}
	return &Middleware{config: config}
}

func (m *Middleware) RequiresPermissions(permissions []string, opts ...Option) fiber.Handler {
	options := optionsDefault(opts...)
	return func(c fiber.Ctx) error {
		if len(permissions) == 0 {
			return c.Next()
		}
		sub := m.config.Lookup(c)
		if sub == "" {
			return m.config.Unauthorized(c)
		}
		switch options.ValidationRule {
		case MatchAllRule:
			for _, permission := range permissions {
				vals := append([]string{sub}, options.PermissionParser(permission)...)
				ok, err := m.config.Enforcer.Enforce(stringSliceToInterfaceSlice(vals)...)
				if err != nil {
					return c.SendStatus(fiber.StatusInternalServerError)
				}
				if !ok {
					return m.config.Forbidden(c)
				}
			}
			return c.Next()
		case AtLeastOneRule:
			for _, permission := range permissions {
				vals := append([]string{sub}, options.PermissionParser(permission)...)
				ok, err := m.config.Enforcer.Enforce(stringSliceToInterfaceSlice(vals)...)
				if err != nil {
					return c.SendStatus(fiber.StatusInternalServerError)
				}
				if ok {
					return c.Next()
				}
			}
			return m.config.Forbidden(c)
		default:
			return c.Next()
		}
	}
}

func (m *Middleware) RoutePermission() fiber.Handler {
	return func(c fiber.Ctx) error {
		sub := m.config.Lookup(c)
		if sub == "" {
			return m.config.Unauthorized(c)
		}
		ok, err := m.config.Enforcer.Enforce(sub, c.Path(), c.Method())
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		if !ok {
			return m.config.Forbidden(c)
		}
		return c.Next()
	}
}

func (m *Middleware) RequiresRoles(roles []string, opts ...Option) fiber.Handler {
	options := optionsDefault(opts...)
	return func(c fiber.Ctx) error {
		if len(roles) == 0 {
			return c.Next()
		}
		sub := m.config.Lookup(c)
		if sub == "" {
			return m.config.Unauthorized(c)
		}
		userRoles, err := m.config.Enforcer.GetRolesForUser(sub)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		switch options.ValidationRule {
		case MatchAllRule:
			for _, role := range roles {
				if !containsString(userRoles, role) {
					return m.config.Forbidden(c)
				}
			}
			return c.Next()
		case AtLeastOneRule:
			for _, role := range roles {
				if containsString(userRoles, role) {
					return c.Next()
				}
			}
			return m.config.Forbidden(c)
		default:
			return c.Next()
		}
	}
}

func optionsDefault(opts ...Option) Options {
	cfg := OptionsDefault
	for _, opt := range opts {
		opt.apply(&cfg)
	}
	return cfg
}

func stringSliceToInterfaceSlice(vals []string) []interface{} {
	items := make([]interface{}, 0, len(vals))
	for _, val := range vals {
		items = append(items, val)
	}
	return items
}

func containsString(vals []string, target string) bool {
	for _, val := range vals {
		if val == target {
			return true
		}
	}
	return false
}
