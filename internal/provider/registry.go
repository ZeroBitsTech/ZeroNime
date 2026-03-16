package provider

import (
	"fmt"
	"strings"
)

const DefaultActiveProvider = "otakudesu"

type Registry struct {
	providers map[string]Provider
}

func NewRegistry(providers ...Provider) *Registry {
	items := make(map[string]Provider, len(providers))
	for _, item := range providers {
		if item == nil {
			continue
		}
		items[strings.ToLower(strings.TrimSpace(item.Name()))] = item
	}
	return &Registry{providers: items}
}

func (r *Registry) Select(name string) (Provider, error) {
	if r == nil {
		return nil, fmt.Errorf("provider registry is nil")
	}

	selected := strings.ToLower(strings.TrimSpace(name))
	if selected == "" {
		selected = DefaultActiveProvider
	}

	item, ok := r.providers[selected]
	if !ok {
		return nil, fmt.Errorf("provider %q is not registered", selected)
	}
	return item, nil
}
