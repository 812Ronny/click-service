package service

import (
	"TestProject1/db"
	"TestProject1/models"
	"time"

	"github.com/pkg/errors"
)

type StatsService struct {
	Repo db.ClickRepository
}

func (s *StatsService) GetClickStats(bannerID int, tsFrom, tsTo time.Time) ([]models.ClickStat, error) {
	stats, err := s.Repo.GetClickStats(bannerID, tsFrom, tsTo)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch click stats for banner %d between %v and %v", bannerID, tsFrom, tsTo)
	}
	return stats, nil
}
