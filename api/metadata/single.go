// Package metadata handles API request that search for metadata regarding osu!
// beatmaps.
package metadata

import (
	"strconv"
	"strings"

	"github.com/osukurikku/cheesegull/dbmirror"

	"github.com/osukurikku/cheesegull/api"
	"github.com/osukurikku/cheesegull/models"
)

// Beatmap handles requests to retrieve single beatmaps.
func Beatmap(c *api.Context) {
	id, _ := strconv.Atoi(strings.TrimSuffix(c.Param("id"), ".json"))
	if id == 0 {
		c.WriteJSON(404, nil)
		return
	}

	bms, err := models.FetchBeatmaps(c.DB, id)
	if err != nil {
		c.Err(err)
		c.WriteJSON(500, nil)
		return
	}
	if len(bms) == 0 {
		c.WriteJSON(404, nil)
		return
	}

	c.WriteJSON(200, bms[0])
}

// Set handles requests to retrieve single beatmap sets.
func Set(c *api.Context) {
	id, _ := strconv.Atoi(strings.TrimSuffix(c.Param("id"), ".json"))
	if id == 0 {
		c.WriteJSON(404, nil)
		return
	}

	set, err := models.FetchSet(c.DB, id, true)
	if err != nil {
		c.Err(err)
		c.WriteJSON(500, nil)
		return
	}
	if set == nil {
		c.WriteJSON(404, nil)
		return
	}

	c.WriteJSON(200, set)
}

// SetMD5 handles requests to retrieve single beatmap set by md5.
func SetMD5(c *api.Context) {
	md5 := strings.TrimSuffix(c.Param("id"), ".json")
	if len(md5) == 0 {
		c.WriteJSON(404, nil)
		return
	}

	set, err := models.FetchSetByMD5(c.DB, md5, true)
	if err != nil {
		c.Err(err)
		c.WriteJSON(500, nil)
		return
	}
	if set == nil {
		c.WriteJSON(404, nil)
		return
	}

	c.WriteJSON(200, set)
}

// RefreshSet handles request for refreshing set
func RefreshSet(c *api.Context) {
	query := c.Request.URL.Query()
	isTokenValid := c.CheckSecret(query.Get("token"))
	if !isTokenValid {
		c.WriteJSON(404, nil)
		return
	}

	id, _ := strconv.Atoi(strings.TrimSuffix(query.Get("id"), ".json"))
	if id == 0 {
		c.WriteJSON(404, nil)
		return
	}

	err := dbmirror.DiscoverOneSet(&c.OsuAPI, c.DB, id)
	if err != nil {
		c.Write([]byte("fuck you leatherman, map not found"))
		return
	}

	c.Write([]byte("okay"))
	return
}

func mustInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func mustPositive(i int) int {
	if i < 0 {
		return 0
	}
	return i
}

func intWithBounds(i, min, max, def int) int {
	if i == 0 {
		return def
	}
	if i < min {
		return min
	}
	if i > max {
		return max
	}
	return i
}

func sIntWithBounds(strs []string, min, max int) []int {
	sInt := make([]int, 0, len(strs))
	for _, s := range strs {
		i, err := strconv.Atoi(s)
		if err != nil || i < min || i > max {
			continue
		}
		sInt = append(sInt, i)
	}
	return sInt
}

// Search does a search on the sets available in the database.
func Search(c *api.Context) {
	query := c.Request.URL.Query()
	sets, err := models.SearchSets(c.DB, c.SearchDB, models.SearchOptions{
		Status: sIntWithBounds(query["status"], -2, 4),
		Query:  query.Get("query"),
		Mode:   sIntWithBounds(query["mode"], 0, 3),

		Amount: intWithBounds(mustInt(query.Get("amount")), 1, 100, 50),
		Offset: mustPositive(mustInt(query.Get("offset"))),
	})
	if err != nil {
		c.Err(err)
		c.WriteJSON(500, nil)
		return
	}

	c.WriteJSON(200, sets)
}

func init() {
	api.GET("/api/b/:id", Beatmap)
	api.GET("/api/md5/:id", SetMD5)
	api.GET("/b/:id", Beatmap)
	api.GET("/api/s/:id", Set)
	api.GET("/s/:id", Set)

	api.GET("/api/search", Search)
	api.GET("/api/update", RefreshSet)
}
