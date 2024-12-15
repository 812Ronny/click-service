package service

import (
	"TestProject1/db"
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type ClickService struct {
	Repo          db.ClickRepository
	Redis         *redis.Client
	incrementChan chan incrementRequest
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

type incrementRequest struct {
	bannerID  int
	timestamp time.Time
}

func NewClickService(repo db.ClickRepository, Redis *redis.Client) *ClickService {
	ctx, cancel := context.WithCancel(context.Background())

	service := &ClickService{
		Repo:          repo,
		Redis:         Redis,
		incrementChan: make(chan incrementRequest, 1000),
		cancel:        cancel,
	}

	service.startProcessing(ctx)
	return service
}

func (s *ClickService) startProcessing(ctx context.Context) {
	s.wg.Add(3)
	go s.processIncrements(ctx)
	go s.syncRedisToPostgreSQL(ctx)
	go s.cleanProcessedKeys(ctx)
}

func (s *ClickService) Stop() {
	s.cancel()
	s.wg.Wait()
	close(s.incrementChan)
}

func (s *ClickService) processIncrements(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-s.incrementChan:
			cacheKey := generateCacheKey(req.bannerID, req.timestamp)

			log.Printf("Processing increment for banner %d at %s", req.bannerID, req.timestamp)

			_, err := s.Redis.Get(ctx, cacheKey).Result()
			if err == redis.Nil {
				err := s.Redis.Set(ctx, cacheKey, 1, 30*time.Minute).Err()
				if err != nil {
					log.Printf("Failed to set click in Redis for banner %d at %s: %v", req.bannerID, req.timestamp, err)
				} else {
					log.Printf("Successfully set click in Redis for banner %d at %s", req.bannerID, req.timestamp)
				}
			} else if err == nil {
				_, err := s.Redis.Incr(ctx, cacheKey).Result()
				if err != nil {
					log.Printf("Failed to increment click in Redis for banner %d at %s: %v", req.bannerID, req.timestamp, err)
				} else {
					log.Printf("Successfully incremented click for banner %d at %s", req.bannerID, req.timestamp)
				}
			} else {
				log.Printf("Error checking Redis cache for banner %d at %s: %v", req.bannerID, req.timestamp, err)
			}
		}
	}
}

func (s *ClickService) IncrementClick(bannerID int, timestamp time.Time) error {
	select {
	case s.incrementChan <- incrementRequest{bannerID: bannerID, timestamp: timestamp}:
		return nil
	default:
		return fmt.Errorf("increment channel is full, dropping click for banner %d", bannerID)
	}
}

func generateCacheKey(bannerID int, timestamp time.Time) string {
	return fmt.Sprintf("banner:%d:%s", bannerID, timestamp.UTC().Format("2006-01-02T15:04:05"))
}

func (s *ClickService) syncRedisToPostgreSQL(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			keys, err := s.fetchKeysFromRedis(ctx, "banner:*")
			if err != nil {
				log.Printf("Error fetching keys from Redis: %v", err)
				continue
			}

			clicksToSync, err := s.groupClicksByKey(ctx, keys)
			if err != nil {
				log.Printf("Error grouping clicks: %v", err)
				continue
			}

			s.syncClicksToPostgreSQL(ctx, clicksToSync)
		}
	}
}

func (s *ClickService) fetchKeysFromRedis(ctx context.Context, pattern string) ([]string, error) {
	keys, _, err := s.Redis.Scan(ctx, 0, pattern, 0).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch keys with pattern '%s': %w", pattern, err)
	}
	return keys, nil
}

func (s *ClickService) processRedisKey(ctx context.Context, key string, re *regexp.Regexp) (string, int, error) {
	if strings.HasPrefix(key, "processed:") {
		return "", 0, nil
	}

	matches := re.FindStringSubmatch(key)
	if len(matches) < 3 {
		return "", 0, fmt.Errorf("failed to parse key '%s'", key)
	}

	countStr, err := s.Redis.Get(ctx, key).Result()
	if err != nil {
		return "", 0, fmt.Errorf("error fetching count for key '%s': %w", key, err)
	}

	clickCount, err := strconv.Atoi(countStr)
	if err != nil {
		return "", 0, fmt.Errorf("error parsing click count for key '%s': %w", key, err)
	}

	groupKey := fmt.Sprintf("%s:%s", matches[1], matches[2])
	return groupKey, clickCount, nil
}

func (s *ClickService) groupClicksByKey(ctx context.Context, keys []string) (map[string][]int, error) {
	re := regexp.MustCompile(`^banner:(\d+):(.+)$`)
	clicksToSync := make(map[string][]int)

	for _, key := range keys {
		groupKey, clickCount, err := s.processRedisKey(ctx, key, re)
		if err != nil {
			log.Printf("Error processing key '%s': %v", key, err)
			continue
		}
		if groupKey != "" {
			clicksToSync[groupKey] = append(clicksToSync[groupKey], clickCount)

			newKey := fmt.Sprintf("processed:%s", key)
			if err := s.Redis.Rename(ctx, key, newKey).Err(); err != nil {
				log.Printf("Failed to rename key '%s' to '%s': %v", key, newKey, err)
			}
		}
	}

	return clicksToSync, nil
}

func (s *ClickService) syncClicksToPostgreSQL(ctx context.Context, clicksToSync map[string][]int) {
	var wg sync.WaitGroup

	for groupKey, clickCounts := range clicksToSync {
		if ctx.Err() != nil {
			log.Println("Context canceled, stopping synchronization")
			return
		}

		wg.Add(1)
		go func(groupKey string, clickCounts []int) {
			defer wg.Done()

			parts := strings.SplitN(strings.TrimSpace(groupKey), ":", 2)
			if len(parts) != 2 {
				log.Printf("Invalid group key format: '%s'", groupKey)
				return
			}

			bannerID, err := strconv.Atoi(parts[0])
			if err != nil {
				log.Printf("Error parsing bannerID from group key '%s': %v", groupKey, err)
				return
			}

			timestamp, err := time.Parse("2006-01-02T15:04:05", parts[1])
			if err != nil {
				log.Printf("Error parsing timestamp from group key '%s': %v", groupKey, err)
				return
			}

			totalClicks := 0
			for _, clickCount := range clickCounts {
				totalClicks += clickCount
			}

			if err := s.Repo.IncrementClick(bannerID, timestamp); err != nil {
				log.Printf("Failed to sync click for banner %d at %v: %v", bannerID, timestamp, err)
			} else {
				log.Printf("Synced %d clicks for banner %d at %v to PostgreSQL", totalClicks, bannerID, timestamp)
			}
		}(groupKey, clickCounts)
	}

	wg.Wait()
}

func (s *ClickService) cleanProcessedKeys(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			keys, _, err := s.Redis.Scan(ctx, 0, "processed:banner:*", 0).Result()
			if err != nil {
				log.Printf("Error fetching processed keys from Redis: %v", err)
				continue
			}

			for _, key := range keys {
				if ttl, _ := s.Redis.TTL(ctx, key).Result(); ttl <= 0 {
					s.Redis.Del(ctx, key)
				}
			}
		}
	}
}
