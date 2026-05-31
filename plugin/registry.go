package plugin

import "sync"

type ExtensionRegistry interface {
	Register(pluginID string, point string, payload any) error
	UnregisterPlugin(pluginID string) error
	List(point string) []Extension
}

type MemoryExtensionRegistry struct {
	mu         sync.RWMutex
	byPoint    map[string][]Extension
	byPluginID map[string][]Extension
}

func NewMemoryExtensionRegistry() *MemoryExtensionRegistry {
	return &MemoryExtensionRegistry{
		byPoint:    make(map[string][]Extension),
		byPluginID: make(map[string][]Extension),
	}
}

func (r *MemoryExtensionRegistry) Register(pluginID string, point string, payload any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ext := Extension{PluginID: pluginID, Point: point, Payload: payload}
	r.byPoint[point] = append(r.byPoint[point], ext)
	r.byPluginID[pluginID] = append(r.byPluginID[pluginID], ext)
	return nil
}

func (r *MemoryExtensionRegistry) UnregisterPlugin(pluginID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	registered := r.byPluginID[pluginID]
	for _, ext := range registered {
		kept := r.byPoint[ext.Point][:0]
		for _, item := range r.byPoint[ext.Point] {
			if item.PluginID != pluginID {
				kept = append(kept, item)
			}
		}
		if len(kept) == 0 {
			delete(r.byPoint, ext.Point)
		} else {
			r.byPoint[ext.Point] = kept
		}
	}
	delete(r.byPluginID, pluginID)
	return nil
}

func (r *MemoryExtensionRegistry) List(point string) []Extension {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := r.byPoint[point]
	out := make([]Extension, len(items))
	copy(out, items)
	return out
}
