package subendpoint

import (
	"context"
	"log"
	"strings"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// ReloadFunc triggers a proxy config reload for the given type.
type ReloadFunc func(proxyType string)

// ProxyStatusFunc returns true if the given proxy type is running.
type ProxyStatusFunc func(ctx context.Context, proxyType string) bool

// Service handles subscription endpoint business logic.
type Service struct {
	repo      *repository.SubscriptionEndpointRepository
	caddyRepo *repository.CaddySiteRepository
	nginxRepo *repository.NginxSiteRepository
	detector  ProxyStatusFunc
	mainPort  string
	reloadFn  ReloadFunc
}

// NewService creates a new subscription endpoint service.
func NewService(
	repo *repository.SubscriptionEndpointRepository,
	caddyRepo *repository.CaddySiteRepository,
	nginxRepo *repository.NginxSiteRepository,
	detector ProxyStatusFunc,
	mainPort string,
	reloadFn ReloadFunc,
) *Service {
	return &Service{
		repo:      repo,
		caddyRepo: caddyRepo,
		nginxRepo: nginxRepo,
		detector:  detector,
		mainPort:  mainPort,
		reloadFn:  reloadFn,
	}
}

// List returns all subscription endpoints.
func (s *Service) List(ctx context.Context) ([]entity.SubscriptionEndpoint, error) {
	return s.repo.List(ctx)
}

// GetByID returns a subscription endpoint by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.SubscriptionEndpoint, error) {
	return s.repo.GetByID(ctx, id)
}

// CreateInput is the input for creating a subscription endpoint.
type CreateInput struct {
	Domain      string `json:"domain" binding:"required"`
	Path        string `json:"path"`
	Enabled     *bool  `json:"enabled"`
	Description string `json:"description"`
}

// Create creates a new subscription endpoint and auto-creates a proxy site.
func (s *Service) Create(ctx context.Context, input CreateInput) (*entity.SubscriptionEndpoint, error) {
	path := normalizePath(input.Path)

	endpoint := &entity.SubscriptionEndpoint{
		Domain:      input.Domain,
		Path:        path,
		Enabled:     true,
		Description: input.Description,
	}
	if input.Enabled != nil {
		endpoint.Enabled = *input.Enabled
	}
	if err := s.repo.Create(ctx, endpoint); err != nil {
		return nil, err
	}

	// Auto-create proxy site for the active proxy
	s.createProxySite(ctx, input.Domain, path, endpoint.Enabled)

	return endpoint, nil
}

// UpdateInput is the input for updating a subscription endpoint.
type UpdateInput struct {
	Domain      *string `json:"domain"`
	Path        *string `json:"path"`
	Enabled     *bool   `json:"enabled"`
	Description *string `json:"description"`
}

// Update updates a subscription endpoint and syncs the proxy site.
func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*entity.SubscriptionEndpoint, error) {
	endpoint, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	oldDomain := endpoint.Domain
	oldPath := endpoint.Path

	if input.Domain != nil {
		endpoint.Domain = *input.Domain
	}
	if input.Path != nil {
		endpoint.Path = normalizePath(*input.Path)
	}
	if input.Enabled != nil {
		endpoint.Enabled = *input.Enabled
	}
	if input.Description != nil {
		endpoint.Description = *input.Description
	}
	if err := s.repo.Update(ctx, endpoint); err != nil {
		return nil, err
	}

	// Sync proxy site
	s.syncProxySite(ctx, oldDomain, oldPath, endpoint.Domain, endpoint.Path, endpoint.Enabled)

	return endpoint, nil
}

// Delete deletes a subscription endpoint and removes the proxy site.
func (s *Service) Delete(ctx context.Context, id int64) error {
	endpoint, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// Remove corresponding proxy site
	s.removeProxySite(ctx, endpoint.Domain, endpoint.Path)

	return nil
}

// getActiveProxy returns "nginx" or "caddy" based on which is running.
func (s *Service) getActiveProxy(ctx context.Context) string {
	if s.detector != nil {
		if s.detector(ctx, "nginx") {
			return "nginx"
		}
		if s.detector(ctx, "caddy") {
			return "caddy"
		}
	}
	// Default: try nginx first
	return "nginx"
}

// createProxySite creates a site in the active proxy.
func (s *Service) createProxySite(ctx context.Context, domain, path string, enabled bool) {
	upstream := "127.0.0.1:" + s.mainPort
	proxyType := s.getActiveProxy(ctx)

	if proxyType == "nginx" && s.nginxRepo != nil {
		site := &entity.NginxSite{
			Domain:   domain,
			Path:     path,
			Upstream: upstream,
			Enabled:  enabled,
			Priority: 100,
		}
		if err := s.nginxRepo.Create(ctx, site); err != nil {
			log.Printf("subendpoint: failed to create nginx site: %v", err)
		} else {
			log.Printf("subendpoint: created nginx site %s%s -> %s", domain, path, upstream)
			s.triggerReload("nginx")
		}
	} else if s.caddyRepo != nil {
		site := &entity.CaddySite{
			Domain:   domain,
			Path:     path,
			Upstream: upstream,
			Enabled:  enabled,
			Priority: 100,
		}
		if err := s.caddyRepo.Create(ctx, site); err != nil {
			log.Printf("subendpoint: failed to create caddy site: %v", err)
		} else {
			log.Printf("subendpoint: created caddy site %s%s -> %s", domain, path, upstream)
			s.triggerReload("caddy")
		}
	}
}

// syncProxySite finds and updates (or creates) the proxy site.
func (s *Service) syncProxySite(ctx context.Context, oldDomain, oldPath, newDomain, newPath string, enabled bool) {
	upstream := "127.0.0.1:" + s.mainPort
	proxyType := s.getActiveProxy(ctx)

	if proxyType == "nginx" && s.nginxRepo != nil {
		sites, err := s.nginxRepo.List(ctx)
		if err == nil {
			for _, site := range sites {
				if site.Domain == oldDomain && normalizePath(site.Path) == oldPath {
					site.Domain = newDomain
					site.Path = newPath
					site.Upstream = upstream
					site.Enabled = enabled
					if err := s.nginxRepo.Update(ctx, &site); err != nil {
						log.Printf("subendpoint: failed to update nginx site: %v", err)
					} else {
						log.Printf("subendpoint: updated nginx site %s%s", newDomain, newPath)
						s.triggerReload("nginx")
					}
					return
				}
			}
		}
		// Not found, create new
		s.createProxySite(ctx, newDomain, newPath, enabled)
	} else if s.caddyRepo != nil {
		sites, err := s.caddyRepo.List(ctx)
		if err == nil {
			for _, site := range sites {
				if site.Domain == oldDomain && normalizePath(site.Path) == oldPath {
					site.Domain = newDomain
					site.Path = newPath
					site.Upstream = upstream
					site.Enabled = enabled
					if err := s.caddyRepo.Update(ctx, &site); err != nil {
						log.Printf("subendpoint: failed to update caddy site: %v", err)
					} else {
						log.Printf("subendpoint: updated caddy site %s%s", newDomain, newPath)
						s.triggerReload("caddy")
					}
					return
				}
			}
		}
		// Not found, create new
		s.createProxySite(ctx, newDomain, newPath, enabled)
	}
}

// removeProxySite removes the proxy site matching domain+path.
func (s *Service) removeProxySite(ctx context.Context, domain, path string) {
	proxyType := s.getActiveProxy(ctx)

	if proxyType == "nginx" && s.nginxRepo != nil {
		sites, err := s.nginxRepo.List(ctx)
		if err == nil {
			for _, site := range sites {
				if site.Domain == domain && normalizePath(site.Path) == normalizePath(path) {
					if err := s.nginxRepo.Delete(ctx, site.ID); err != nil {
						log.Printf("subendpoint: failed to delete nginx site: %v", err)
					} else {
						log.Printf("subendpoint: removed nginx site %s%s", domain, path)
						s.triggerReload("nginx")
					}
					return
				}
			}
		}
	} else if s.caddyRepo != nil {
		sites, err := s.caddyRepo.List(ctx)
		if err == nil {
			for _, site := range sites {
				if site.Domain == domain && normalizePath(site.Path) == normalizePath(path) {
					if err := s.caddyRepo.Delete(ctx, site.ID); err != nil {
						log.Printf("subendpoint: failed to delete caddy site: %v", err)
					} else {
						log.Printf("subendpoint: removed caddy site %s%s", domain, path)
						s.triggerReload("caddy")
					}
					return
				}
			}
		}
	}
}

// triggerReload calls the reload function if set.
func (s *Service) triggerReload(proxyType string) {
	if s.reloadFn != nil {
		s.reloadFn(proxyType)
	}
}

// normalizePath ensures path starts with / and doesn't end with / (except root).
func normalizePath(p string) string {
	if p == "" || p == "/" {
		return ""
	}
	p = strings.TrimRight(p, "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
