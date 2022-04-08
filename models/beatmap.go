package models

import (
	"database/sql"
	"fmt"
)

// Beatmap represents a single beatmap (difficulty) on osu!.
type Beatmap struct {
	ID               int `json:"BeatmapID"`
	ParentSetID      int
	DiffName         string
	FileMD5          string
	Mode             int
	BPM              float64
	AR               float32
	OD               float32
	CS               float32
	HP               float32
	TotalLength      int
	HitLength        int
	Playcount        int
	Passcount        int
	MaxCombo         int
	DifficultyRating float64
}

type BeatmapChimu struct {
	ID               int `json:"BeatmapId"`
	ParentSetId      int
	DiffName         string
	FileMD5          string
	Mode             int
	BPM              float64
	AR               float32
	OD               float32
	CS               float32
	HP               float32
	TotalLength      int
	HitLength        int
	Playcount        int
	Passcount        int
	MaxCombo         int
	DifficultyRating float64
	OsuFile          string
	DownloadPath     string
}

const beatmapFields = `
beatmaps.id, beatmaps.parent_set_id, beatmaps.diff_name, beatmaps.file_md5, beatmaps.mode, beatmaps.bpm,
beatmaps.ar, beatmaps.od, beatmaps.cs, beatmaps.hp, beatmaps.total_length, beatmaps.hit_length,
beatmaps.playcount, beatmaps.passcount, beatmaps.max_combo, beatmaps.difficulty_rating`

func readBeatmapsFromRows(rows *sql.Rows, capacity int) ([]Beatmap, error) {
	var err error
	bms := make([]Beatmap, 0, capacity)
	for rows.Next() {
		var b Beatmap
		err = rows.Scan(
			&b.ID, &b.ParentSetID, &b.DiffName, &b.FileMD5, &b.Mode, &b.BPM,
			&b.AR, &b.OD, &b.CS, &b.HP, &b.TotalLength, &b.HitLength,
			&b.Playcount, &b.Passcount, &b.MaxCombo, &b.DifficultyRating,
		)
		if err != nil {
			return nil, err
		}
		bms = append(bms, b)
	}

	return bms, rows.Err()
}

func readBeatmapsFromRowsChimu(rows *sql.Rows, capacity int) ([]BeatmapChimu, error) {
	var err error
	bms_chimu := make([]BeatmapChimu, 0, capacity)
	for rows.Next() {
		var bcm BeatmapChimu
		var artist, title, creator string

		err = rows.Scan(
			&bcm.ID, &bcm.ParentSetId, &bcm.DiffName, &bcm.FileMD5, &bcm.Mode, &bcm.BPM,
			&bcm.AR, &bcm.OD, &bcm.CS, &bcm.HP, &bcm.TotalLength, &bcm.HitLength,
			&bcm.Playcount, &bcm.Passcount, &bcm.MaxCombo, &bcm.DifficultyRating, &artist, &title, &creator,
		)

		if err != nil {
			return nil, err
		}

		bcm.OsuFile = fmt.Sprintf("%s - %s (%s) [%s].osu", artist, title, creator, bcm.DiffName)
		bcm.DownloadPath = fmt.Sprintf("/d/%d", bcm.ParentSetId)
		bms_chimu = append(bms_chimu, bcm)
	}
	return bms_chimu, rows.Err()
}

func inClause(length int) string {
	if length <= 0 {
		return ""
	}
	b := make([]byte, length*3-2)
	for i := 0; i < length; i++ {
		b[i*3] = '?'
		if i != length-1 {
			b[i*3+1] = ','
			b[i*3+2] = ' '
		}
	}
	return string(b)
}

func sIntToSInterface(i []int) []interface{} {
	args := make([]interface{}, len(i))
	for idx, id := range i {
		args[idx] = id
	}
	return args
}

// FetchBeatmaps retrieves a list of beatmap knowing their IDs.
func FetchBeatmaps(db *sql.DB, ids ...int) ([]Beatmap, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	q := `SELECT ` + beatmapFields + ` FROM beatmaps WHERE id IN (` + inClause(len(ids)) + `)`

	rows, err := db.Query(q, sIntToSInterface(ids)...)
	if err != nil {
		return nil, err
	}

	return readBeatmapsFromRows(rows, len(ids))
}

// FetchBeatmaps retrieves a list of beatmap knowing their IDs.
func FetchBeatmapsByMd5(db *sql.DB, md5 string) ([]Beatmap, error) {
	if len(md5) == 0 {
		return nil, nil
	}

	q := `SELECT ` + beatmapFields + ` FROM beatmaps WHERE file_md5 = ? `

	rows, err := db.Query(q, md5)
	if err != nil {
		return nil, err
	}

	return readBeatmapsFromRows(rows, 1)
}

// FetchBeatmaps retrieves a list of beatmap knowing their IDs.
func FetchBeatmapsChimu(db *sql.DB, ids ...int) ([]BeatmapChimu, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	q := `SELECT ` + beatmapFields + `, sets.artist, sets.title, sets.creator FROM beatmaps RIGHT JOIN sets ON sets.id = beatmaps.parent_set_id WHERE beatmaps.id IN (` + inClause(len(ids)) + `)`

	rows, err := db.Query(q, sIntToSInterface(ids)...)
	if err != nil {
		return nil, err
	}

	return readBeatmapsFromRowsChimu(rows, len(ids))
}

// CreateBeatmaps adds beatmaps in the database.
func CreateBeatmaps(db *sql.DB, bms ...Beatmap) error {
	if len(bms) == 0 {
		return nil
	}

	q := `INSERT IGNORE INTO beatmaps(` + beatmapFields + `) VALUES `
	const valuePlaceholder = `(
		?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?,
		?, ?, ?, ?
	)`

	args := make([]interface{}, 0, 15*4)
	for idx, bm := range bms {
		if idx != 0 {
			q += ", "
		}
		q += valuePlaceholder
		args = append(args,
			bm.ID, bm.ParentSetID, bm.DiffName, bm.FileMD5, bm.Mode, bm.BPM,
			bm.AR, bm.OD, bm.CS, bm.HP, bm.TotalLength, bm.HitLength,
			bm.Playcount, bm.Passcount, bm.MaxCombo, bm.DifficultyRating,
		)
	}

	_, err := db.Exec(q, args...)
	return err
}
