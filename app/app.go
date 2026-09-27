// Package app provides compile-time registration for source feature sets.
package app

import (
	"sort"
	"sync"

	"github.com/gofiber/fiber/v3"
)

type Auth string

const (
	AuthPublic Auth = "public"
	AuthUser   Auth = "user"
	AuthAdmin  Auth = "admin"
)

type PermissionType string

const (
	PermMenu   PermissionType = "menu"
	PermButton PermissionType = "button"
	PermAPI    PermissionType = "api"
)

type Route struct {
	Method  string
	Path    string
	Auth    Auth
	Handler fiber.Handler
}

type ConfigItem struct {
	Key    string
	Name   string
	Value  string
	Type   string
	Group  string
	Remark string
}

type Permission struct {
	Type        PermissionType
	Name        string
	Code        string
	Path        string
	Method      string
	MenuPath    string
	Icon        string
	Component   string
	Parent      string
	SortOrder   int
	IsCommon    bool
	Description string
}

type App struct {
	Code        string
	Name        string
	Version     string
	Enabled     bool
	Config      []ConfigItem
	Routes      []Route
	Permissions []Permission
}

type Options struct {
	// BasePath is the path of the supplied router, used to match API permissions.
	BasePath      string
	ReservedCodes []string
	Middleware    func(Auth) []fiber.Handler
}

var (
	registryMu sync.RWMutex
	registry   []App
)

// Register adds a feature set to the process. Validate checks declarations before startup.
func Register(a App) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, a)
}

// Registered returns a stable snapshot sorted by code.
func Registered() []App {
	registryMu.RLock()
	defer registryMu.RUnlock()
	result := append([]App(nil), registry...)
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	return result
}
