package api

import (
	"encoding/json"

	"github.com/osuripple/cheesegull/models"
)

type statusJSON struct {
	MaxSize           uint64 `json:max_cache_size`
	MaxSizeInGB       int    `json:max_cache_size_gb`
	SessionMapsLength int    `json:session_maps_length`
	BiggestSetID      int    `json:biggest_set_id`
}

func statusHandler(c *Context) {
	c.WriteHeader("Content-Type", "application/json; charset=utf-8")
	biggestSetID, err := models.BiggestSetID(c.DB)
	if err != nil {
		biggestSetID = 0
	}

	status := statusJSON{
		MaxSize:           c.House.MaxSize,
		MaxSizeInGB:       c.House.MaxSizeGB,
		SessionMapsLength: len(c.House.State),
		BiggestSetID:      biggestSetID,
	}

	jsoned, _ := json.Marshal(status)
	c.Write([]byte(jsoned))
}

func init() {
	GET("/status", statusHandler)
}
