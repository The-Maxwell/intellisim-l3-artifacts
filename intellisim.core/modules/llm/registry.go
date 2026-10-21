package llm

import (
	"sync"

	"go.uber.org/zap"
	"intelli.sim.core/data/common"
)

// Registry tracks connected providers, keyed by provider UID.
type Registry interface {
	Register(provider Provider)
	Unregister(uid string)
	Get(uid string) (Provider, bool)
	List() []ProviderInfo
	ListByCapability(capability common.ModelCapability) []ProviderInfo
	ListByModel(modelUID string) []ProviderInfo
}

type registry struct {
	logger    *zap.Logger
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewRegistry(logger *zap.Logger) Registry {
	logger.Info("LLM registry initialized, waiting for providers to connect via gRPC")
	return &registry{
		logger:    logger.Named("llm-registry"),
		providers: make(map[string]Provider),
	}
}

func (r *registry) Register(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.UID()] = provider
	r.logger.Info("LLM provider registered",
		zap.String("uid", provider.UID()),
		zap.Int("models", len(provider.Info().Models)),
	)
}

func (r *registry) Unregister(uid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.providers[uid]; ok {
		delete(r.providers, uid)
		r.logger.Info("LLM provider unregistered", zap.String("uid", uid))
	}
}

func (r *registry) Get(uid string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[uid]
	return p, ok
}

func (r *registry) List() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProviderInfo, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p.Info())
	}
	return out
}

// ListByCapability returns providers having at least one model with the given
// capability; the returned infos contain only the matching models.
func (r *registry) ListByCapability(capability common.ModelCapability) []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ProviderInfo
	for _, p := range r.providers {
		info := p.Info()
		var models []ModelInfo
		for _, m := range info.Models {
			for _, c := range m.Capabilities {
				if c == capability {
					models = append(models, m)
					break
				}
			}
		}
		if len(models) > 0 {
			info.Models = models
			out = append(out, info)
		}
	}
	return out
}

// ListByModel returns providers offering the given model UID; the returned
// infos contain only that model.
func (r *registry) ListByModel(modelUID string) []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ProviderInfo
	for _, p := range r.providers {
		info := p.Info()
		var models []ModelInfo
		for _, m := range info.Models {
			if m.UID == modelUID {
				models = append(models, m)
			}
		}
		if len(models) > 0 {
			info.Models = models
			out = append(out, info)
		}
	}
	return out
}
