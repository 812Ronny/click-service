package db

import (
	"TestProject1/models"
	"time"

	"github.com/pkg/errors"
)

type ClickRepository interface {
	IncrementClick(bannerID int, timestamp time.Time) error
	GetClickStats(bannerID int, tsFrom, tsTo time.Time) ([]models.ClickStat, error)
}

type PSQLClickRepository struct {
	DB *DB
}

func (r *PSQLClickRepository) IncrementClick(bannerID int, timestamp time.Time) error {
	query := `INSERT INTO clicks (timestamp, banner_id, count)
              VALUES ($1, $2, 1)
              ON CONFLICT (timestamp, banner_id)
              DO UPDATE SET count = clicks.count + 1`
	_, err := r.DB.Conn.Exec(query, timestamp, bannerID)
	if err != nil {
		return errors.Wrapf(err, "failed to increment click for banner %d at %v", bannerID, timestamp)
	}
	return nil
}

func (r *PSQLClickRepository) GetClickStats(bannerID int, tsFrom, tsTo time.Time) ([]models.ClickStat, error) {
	query := `SELECT timestamp, banner_id, count FROM clicks
              WHERE banner_id = $1 AND timestamp BETWEEN $2 AND $3`
	rows, err := r.DB.Conn.Query(query, bannerID, tsFrom, tsTo)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch click stats for banner %d between %v and %v", bannerID, tsFrom, tsTo)
	}
	defer rows.Close()

	var stats []models.ClickStat
	for rows.Next() {
		var stat models.ClickStat
		if err := rows.Scan(&stat.Timestamp, &stat.BannerID, &stat.Count); err != nil {
			return nil, errors.Wrapf(err, "failed to scan row for banner %d", bannerID)
		}
		stats = append(stats, stat)
	}

	return stats, nil
}
