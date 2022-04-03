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

type ChimuAnswer struct {
	Data    interface{} `json:"data"`
	Code    int         `json:"code"`
	Message string      `json:"message"`
}

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

// BeatmapChimu handles requests to retrieve single beatmaps.
func BeatmapChimu(c *api.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		c.WriteJSON(404, nil)
		return
	}

	bms, err := models.FetchBeatmapsChimu(c.DB, id)
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

// BeatmapChimu handles requests to retrieve single beatmaps.
func BeatmapMd5(c *api.Context) {
	md5 := c.Param("id")
	if len(md5) == 0 {
		c.WriteJSON(404, nil)
		return
	}

	bms, err := models.FetchBeatmapsByMd5(c.DB, md5)
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

// SetChimu handles requests to retrieve single beatmap sets.
func SetChimu(c *api.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		c.WriteJSON(404, nil)
		return
	}

	set, err := models.FetchSetChimu(c.DB, id, true)
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
func mustFloat32(s string) float32 {
	i, _ := strconv.ParseFloat(s, 32)
	return float32(i)
}
func mustFloat64(s string) float64 {
	i, _ := strconv.ParseFloat(s, 64)
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

func float32WithBounds(i, min, max, def float32) float32 {
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

func float64WithBounds(i, min, max, def float64) float64 {
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

// Search does a search on the sets available in the database.
func SearchChimu(c *api.Context) {
	query := c.Request.URL.Query()
	sets, err := models.SearchSetsChimu(c.DB, c.SearchDB, models.SearchOptions{
		Status: sIntWithBounds(query["status"], -2, 4),
		Query:  query.Get("query"),
		Mode:   sIntWithBounds(query["mode"], 0, 3),

		Amount: intWithBounds(mustInt(query.Get("amount")), 1, 100, 50),
		Offset: mustPositive(mustInt(query.Get("offset"))),

		MinAR:               float32WithBounds(mustFloat32(query.Get("min_ar")), 0, 10, -1),
		MaxAR:               float32WithBounds(mustFloat32(query.Get("max_ar")), 0, 10, -1),
		MinOD:               float32WithBounds(mustFloat32(query.Get("min_od")), 0, 10, -1),
		MaxOD:               float32WithBounds(mustFloat32(query.Get("max_od")), 0, 10, -1),
		MinCS:               float32WithBounds(mustFloat32(query.Get("min_cs")), 0, 10, -1),
		MaxCS:               float32WithBounds(mustFloat32(query.Get("max_cs")), 0, 10, -1),
		MinHP:               float32WithBounds(mustFloat32(query.Get("min_hp")), 0, 10, -1),
		MaxHP:               float32WithBounds(mustFloat32(query.Get("max_hp")), 0, 10, -1),
		MinDifficultyRating: float64WithBounds(mustFloat64(query.Get("min_diff")), 0, 10, -1),
		MaxDifficultyRating: float64WithBounds(mustFloat64(query.Get("max_diff")), 0, 10, -1),
		MinTotalLength:      intWithBounds(mustInt(query.Get("min_length")), 0, 10, -1),
		MaxTotalLength:      intWithBounds(mustInt(query.Get("max_length")), 0, 10, -1),

		MinBPM:   float64WithBounds(mustFloat64(query.Get("min_bpm")), 0, 999, -1),
		MaxBPM:   float64WithBounds(mustFloat64(query.Get("max_bpm")), 0, 999, -1),
		Genre:    mustPositive(mustInt(query.Get("genre"))),
		Language: mustPositive(mustInt(query.Get("language"))),
	})
	if err != nil {
		c.Err(err)
		c.WriteJSON(500, ChimuAnswer{
			Code:    500,
			Message: "Something bad happend",
		})
		return
	}

	c.WriteJSON(200, ChimuAnswer{
		Data:    sets,
		Message: "",
		Code:    200,
	})
}

func init() {
	api.GET("/api/b/:id", Beatmap)
	api.GET("/api/md5/:id", BeatmapMd5)
	api.GET("/b/:id", Beatmap)
	api.GET("/api/s/:id", Set)
	api.GET("/s/:id", Set)

	api.GET("/api/search", Search)
	api.GET("/api/update", RefreshSet)

	// Chimu compatibility
	api.GET("/api/v1/map/:id", BeatmapChimu)
	api.GET("/api/v1/set/:id", SetChimu)
	api.GET("/api/v1/search", SearchChimu)
}
