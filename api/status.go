package api

import (
	"encoding/json"

	"github.com/osukurikku/cheesegull/models"
)

type statusJSON struct {
	MaxSize         uint64 `json:max_cache_size`
	MaxSizeInGB     int    `json:max_cache_size_gb`
	CacheMapsLength int    `json:cache_maps_length`
	CacheMapsSize   uint64 `json:cache_maps_size`
	CountMaps       int    `json:count_maps`
	BiggestSetID    int    `json:biggest_set_id`
}

func statusHandler(c *Context) {
	c.WriteHeader("Content-Type", "application/json; charset=utf-8")
	biggestSetID, err := models.BiggestSetID(c.DB)
	if err != nil {
		biggestSetID = 0
	}

	var countMaps int
	row, _ := c.DB.Query("SELECT COUNT(*) FROM sets")
	for row.Next() {
		_ = row.Scan(&countMaps)
	}

	totalSize, _ := c.House.StateSizeAndRemovableMaps()

	status := statusJSON{
		MaxSize:         c.House.MaxSize,
		MaxSizeInGB:     c.House.MaxSizeGB,
		CacheMapsLength: len(c.House.State),
		CacheMapsSize:   totalSize / 1024 / 1024 / 1024,
		CountMaps:       countMaps,
		BiggestSetID:    biggestSetID,
	}

	jsoned, _ := json.Marshal(status)
	c.Write([]byte(jsoned))
}

func init() {
	GET("/status", statusHandler)
}
