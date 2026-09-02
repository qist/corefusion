package cfip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles CF IP business logic.
type Service struct {
	sourceRepo *repository.CFIPSourceRepository
	entryRepo  *repository.CFIPEntryRepository
	httpClient *http.Client
}

// NewService creates a new CF IP service.
func NewService(sourceRepo *repository.CFIPSourceRepository, entryRepo *repository.CFIPEntryRepository) *Service {
	return &Service{
		sourceRepo: sourceRepo,
		entryRepo:  entryRepo,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ListSources returns all CF IP sources.
func (s *Service) ListSources(ctx context.Context) ([]entity.CFIPSource, error) {
	return s.sourceRepo.List(ctx)
}

// ListSourcesPaged returns CF IP sources with pagination and total count.
func (s *Service) ListSourcesPaged(ctx context.Context, limit, offset int) ([]entity.CFIPSource, int64, error) {
	items, err := s.sourceRepo.ListPaged(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.sourceRepo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CreateSource creates a new CF IP source.
func (s *Service) CreateSource(ctx context.Context, name, url, sourceType, ports string, interval int) (*entity.CFIPSource, error) {
	if interval <= 0 {
		interval = 3600
	}
	if ports == "" {
		if sourceType == "bestproxy" {
			ports = "[443]"
		} else {
			ports = "[80,443,8080,8443,8880,2052,2053,2082,2083,2086,2087,2095,2096]"
		}
	}
	source := &entity.CFIPSource{
		Name:     name,
		URL:      url,
		Type:     sourceType,
		Ports:    ports,
		Interval: interval,
		Enabled:  true,
	}
	source.CreatedAt = time.Now()
	source.UpdatedAt = time.Now()
	if err := s.sourceRepo.Create(ctx, source); err != nil {
		return nil, err
	}
	return source, nil
}

// UpdateSource updates a CF IP source.
func (s *Service) UpdateSource(ctx context.Context, id int64, name, url, sourceType, ports string, interval int, enabled bool) (*entity.CFIPSource, error) {
	source, err := s.sourceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		source.Name = name
	}
	if url != "" {
		source.URL = url
	}
	if sourceType != "" {
		source.Type = sourceType
	}
	if ports != "" {
		source.Ports = ports
	}
	if interval > 0 {
		source.Interval = interval
	}
	source.Enabled = enabled
	source.UpdatedAt = time.Now()
	if err := s.sourceRepo.Update(ctx, source); err != nil {
		return nil, err
	}
	return source, nil
}

// DeleteSource deletes a CF IP source and its entries.
func (s *Service) DeleteSource(ctx context.Context, id int64) error {
	if err := s.entryRepo.DeleteBySource(ctx, id); err != nil {
		return err
	}
	return s.sourceRepo.Delete(ctx, id)
}

// ListEntries returns all CF IP entries.
func (s *Service) ListEntries(ctx context.Context, sourceID int64) ([]entity.CFIPEntry, error) {
	return s.entryRepo.List(ctx, sourceID)
}

// ListEntriesPaged returns CF IP entries filtered by source with pagination and total count.
func (s *Service) ListEntriesPaged(ctx context.Context, sourceID int64, limit, offset int) ([]entity.CFIPEntry, int64, error) {
	items, err := s.entryRepo.ListPaged(ctx, sourceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.entryRepo.CountBySource(ctx, sourceID)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// AddEntry adds a manual CF IP entry.
func (s *Service) AddEntry(ctx context.Context, ip, ports string) (*entity.CFIPEntry, error) {
	if ports == "" {
		ports = "[80,443,8080,8443,8880,2052,2053,2082,2083,2086,2087,2095,2096]"
	}
	entry := &entity.CFIPEntry{
		SourceID: 0, // manual
		IP:       ip,
		Ports:    ports,
		Enabled:  true,
	}
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	if err := s.entryRepo.Create(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// DeleteEntry deletes a CF IP entry.
func (s *Service) DeleteEntry(ctx context.Context, id int64) error {
	return s.entryRepo.Delete(ctx, id)
}

// ClearEntries clears all CF IP entries.
func (s *Service) ClearEntries(ctx context.Context) error {
	return s.entryRepo.ClearAll(ctx)
}

// FetchFromSource fetches IPs from a source.
func (s *Service) FetchFromSource(ctx context.Context, sourceID int64) error {
	source, err := s.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		return err
	}

	// Delete old entries for this source
	if err := s.entryRepo.DeleteBySource(ctx, sourceID); err != nil {
		return err
	}

	// Fetch IPs from URL
	ips, err := s.fetchIPs(source.URL)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// Parse ports
	var ports []int
	if source.Ports != "" {
		json.Unmarshal([]byte(source.Ports), &ports)
	}
	if len(ports) == 0 {
		if source.Type == "bestproxy" {
			ports = []int{443}
		} else {
			ports = []int{80, 443, 8080, 8443, 8880, 2052, 2053, 2082, 2083, 2086, 2087, 2095, 2096}
		}
	}
	portsJSON, _ := json.Marshal(ports)

	// Create entries
	for _, ip := range ips {
		entry := &entity.CFIPEntry{
			SourceID: sourceID,
			IP:       ip,
			Ports:    string(portsJSON),
			Enabled:  true,
		}
		entry.CreatedAt = time.Now()
		entry.UpdatedAt = time.Now()
		s.entryRepo.Create(ctx, entry)
	}

	// Update last run time
	source.LastRun = time.Now()
	s.sourceRepo.Update(ctx, source)

	log.Printf("cfip: fetched %d IPs from %s", len(ips), source.Name)
	return nil
}

// FetchAllEnabled fetches IPs from all enabled sources.
func (s *Service) FetchAllEnabled(ctx context.Context) error {
	sources, err := s.sourceRepo.GetEnabled(ctx)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if err := s.FetchFromSource(ctx, source.ID); err != nil {
			log.Printf("cfip: fetch from %s failed: %v", source.Name, err)
		}
	}
	return nil
}

// fetchIPs fetches IP list from URL.
func (s *Service) fetchIPs(url string) ([]string, error) {
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	content := string(body)
	return extractIPs(content), nil
}

// extractIPs extracts IP addresses from text.
func extractIPs(text string) []string {
	// Match IPv4 addresses
	re := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	matches := re.FindAllString(text, -1)

	// Deduplicate
	seen := make(map[string]bool)
	var result []string
	for _, ip := range matches {
		if !seen[ip] {
			seen[ip] = true
			result = append(result, ip)
		}
	}
	return result
}

// SeedDefaultSources seeds default CF IP sources.
func (s *Service) SeedDefaultSources(ctx context.Context) error {
	sources, err := s.sourceRepo.List(ctx)
	if err != nil {
		return err
	}
	if len(sources) > 0 {
		return nil
	}

	defaults := []entity.CFIPSource{
		{Name: "BestCF", URL: "https://ipdb.api.030101.xyz/?type=bestcf&country=true", Type: "bestcf", Ports: "[443]", Interval: 3600, Enabled: true},
		{Name: "CloudFlareYes", URL: "https://addressesapi.090227.xyz/CloudFlareYes", Type: "bestcf", Ports: "[443]", Interval: 3600, Enabled: true},
		{Name: "Official CF", URL: "https://addressesapi.090227.xyz/ip.164746.xyz", Type: "bestcf", Ports: "[443]", Interval: 3600, Enabled: true},
		{Name: "BestProxy", URL: "https://ipdb.api.030101.xyz/?type=bestproxy&country=true", Type: "bestproxy", Ports: "[443]", Interval: 3600, Enabled: true},
	}

	for _, d := range defaults {
		d.CreatedAt = time.Now()
		d.UpdatedAt = time.Now()
		if err := s.sourceRepo.Create(ctx, &d); err != nil {
			return err
		}
	}
	return nil
}

// StartScheduler starts the background scheduler with the given check interval.
func (s *Service) StartScheduler(ctx context.Context, checkInterval time.Duration) {
	if checkInterval <= 0 {
		checkInterval = 5 * time.Minute
	}
	go func() {
		// Run an initial check immediately on startup
		s.checkAndFetch(ctx)

		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.checkAndFetch(ctx)
			}
		}
	}()
}

func (s *Service) checkAndFetch(ctx context.Context) {
	sources, err := s.sourceRepo.GetEnabled(ctx)
	if err != nil {
		log.Printf("cfip: failed to get enabled sources: %v", err)
		return
	}

	now := time.Now()
	for _, source := range sources {
		if now.Sub(source.LastRun) >= time.Duration(source.Interval)*time.Second {
			if err := s.FetchFromSource(ctx, source.ID); err != nil {
				log.Printf("cfip: scheduled fetch from %s failed: %v", source.Name, err)
			}
		}
	}
}
