package models

import "time"

type Banner struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ClickStat struct {
	Timestamp time.Time `json:"timestamp"`
	BannerID  int       `json:"banner_id"`
	Count     int       `json:"count"`
}
