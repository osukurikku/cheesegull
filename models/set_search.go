package models

import (
	"bytes"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// SearchOptions are options that can be passed to SearchSets for filtering
// sets.
type SearchOptions struct {
	// If len is 0, then it should be treated as if all statuses are good.
	Status []int
	Query  string
	// Gamemodes to which limit the results. If len is 0, it means all modes
	// are ok.
	Mode []int

	// Pagination options.
	Offset int
	Amount int

	// Stats optiosn
	MinAR               float32
	MaxAR               float32
	MinOD               float32
	MaxOD               float32
	MinCS               float32
	MaxCS               float32
	MinHP               float32
	MaxHP               float32
	MinDifficultyRating float64
	MaxDifficultyRating float64
	MinTotalLength      int
	MaxTotalLength      int

	// Beatmap additional lols
	MinBPM   float64
	MaxBPM   float64
	Genre    int
	Language int
}

func (o SearchOptions) setModes() (total uint8) {
	for _, m := range o.Mode {
		if m < 0 || m >= 4 {
			continue
		}
		total |= 1 << uint8(m)
	}
	return
}

var mysqlStringReplacer = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	`'`, `\'`,
	"\x00", `\0`,
	"\n", `\n`,
	"\r", `\r`,
	"\x1a", `\Z`,
)

func sIntCommaSeparated(nums []int) string {
	b := bytes.Buffer{}
	for idx, num := range nums {
		b.WriteString(strconv.Itoa(num))
		if idx != len(nums)-1 {
			b.WriteString(", ")
		}
	}
	return b.String()
}

const setFieldsWithRow = `sets.id, sets.ranked_status, sets.approved_date, sets.last_update, sets.last_checked,
sets.artist, sets.title, sets.creator, sets.source, sets.tags, sets.has_video, sets.genre,
language, sets.favourites`

// SearchSets retrieves sets, filtering them using SearchOptions.
func SearchSets(db, searchDB *sql.DB, opts SearchOptions) ([]Set, error) {
	sm := strconv.Itoa(int(opts.setModes()))

	// first, we create the where conditions that are valid for both querying mysql
	// straight or querying sphinx first.
	var whereConds string
	var havingConds string
	if len(opts.Status) != 0 {
		whereConds = "ranked_status IN (" + sIntCommaSeparated(opts.Status) + ") "
	}
	if len(opts.Mode) != 0 {
		// This is a hack. Apparently, Sphinx does not support AND bitwise
		// operations in the WHERE clause, so we're placing that in the SELECT
		// clause and only making sure it's correct in this place.
		havingConds = " valid_set_modes = " + sm + " "
	}

	// Limit user amount for beatmap asking
	if opts.Amount > 100 {
		opts.Amount = 100
	}

	sets := make([]Set, 0, opts.Amount)
	setIDs := make([]int, 0, opts.Amount)
	// setMap is used when a query is given to make sure the results are kept in the correct
	// order given by sphinx.
	setMap := make(map[int]int, opts.Amount)
	// if Sphinx is used, limit will be cleared so that it's not used for the mysql query
	limit := fmt.Sprintf(" LIMIT %d, %d ", opts.Offset, opts.Amount)

	if opts.Query != "" {
		setIDsQuery := "SELECT id, set_modes & " + sm + " AS valid_set_modes FROM cg WHERE "

		// add filters to query
		// Yes. I know. Prepared statements. But Sphinx doesn't like them, so
		// bummer.
		setIDsQuery += "MATCH('" + mysqlStringReplacer.Replace(opts.Query) + "') "
		if whereConds != "" {
			setIDsQuery += "AND " + whereConds
		}
		if havingConds != "" {
			setIDsQuery += " AND " + havingConds
		}
		setIDsQuery = strings.ReplaceAll(setIDsQuery, "sets.", "")

		// set limit
		setIDsQuery += " ORDER BY WEIGHT() DESC, id DESC " + limit + " OPTION ranker=sph04, max_matches=20000 "
		limit = ""

		// fetch rows
		rows, err := searchDB.Query(setIDsQuery)
		if err != nil {
			return nil, err
		}

		// contains IDs of the sets we will retrieve
		for rows.Next() {
			var id int
			err = rows.Scan(&id, new(int))
			if err != nil {
				return nil, err
			}
			setIDs = append(setIDs, id)
			sets = sets[:len(sets)+1]
			setMap[id] = len(sets) - 1
		}

		// short path: there are no sets
		if len(sets) == 0 {
			return []Set{}, nil
		}

		whereConds = "sets.id IN (" + sIntCommaSeparated(setIDs) + ")"
		havingConds = ""
	}

	if whereConds != "" {
		whereConds = "WHERE " + whereConds
	}
	if havingConds != "" {
		havingConds = " HAVING " + havingConds
	}
	setsQuery := "SELECT " + setFieldsWithRow + ", sets.set_modes & " + sm + " AS valid_set_modes FROM sets " +
		whereConds + havingConds + " ORDER BY last_update DESC " + limit
	rows, err := db.Query(setsQuery)

	if err != nil {
		return nil, err
	}

	// find all beatmaps, but leave children aside for the moment.
	for rows.Next() {
		var s Set
		err = rows.Scan(
			&s.ID, &s.RankedStatus, &s.ApprovedDate, &s.LastUpdate, &s.LastChecked,
			&s.Artist, &s.Title, &s.Creator, &s.Source, &s.Tags, &s.HasVideo, &s.Genre,
			&s.Language, &s.Favourites, new(int),
		)
		if err != nil {
			return nil, err
		}
		// we get the position we should place s in from the setMap, this way we
		// keep the order of results as sphinx prefers.
		pos, ok := setMap[s.ID]
		if ok {
			sets[pos] = s
		} else {
			sets = append(sets, s)
			setIDs = append(setIDs, s.ID)
			setMap[s.ID] = len(sets) - 1
		}
	}

	if len(sets) == 0 {
		return sets, nil
	}

	rows, err = db.Query(
		"SELECT "+beatmapFields+" FROM beatmaps WHERE parent_set_id IN ("+
			inClause(len(setIDs))+")",
		sIntToSInterface(setIDs)...,
	)
	if err != nil {
		return nil, err
	}

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
		parentSet, ok := setMap[b.ParentSetID]
		if !ok {
			continue
		}
		sets[parentSet].ChildrenBeatmaps = append(sets[parentSet].ChildrenBeatmaps, b)
	}

	return sets, nil
}

// SearchSetsChimu retrieves sets, filtering them using SearchOptions.
func SearchSetsChimu(db, searchDB *sql.DB, opts SearchOptions) ([]SetChimu, error) {
	sm := strconv.Itoa(int(opts.setModes()))

	// first, we create the where conditions that are valid for both querying mysql
	// straight or querying sphinx first.
	var whereConds string
	var havingConds string
	var beforeWhereConds = " "
	if len(opts.Status) != 0 {
		whereConds = beforeWhereConds + "sets.ranked_status IN (" + sIntCommaSeparated(opts.Status) + ") "
		beforeWhereConds = " AND "
	}
	if opts.Genre > 0 {
		whereConds += fmt.Sprintf(beforeWhereConds+"sets.genre = %d ", opts.Genre)
		beforeWhereConds = " AND "
	}
	if opts.Language > 0 {
		whereConds += fmt.Sprintf(beforeWhereConds+"sets.language = %d ", opts.Language)
		beforeWhereConds = " AND "
	}
	if len(opts.Mode) != 0 {
		// This is a hack. Apparently, Sphinx does not support AND bitwise
		// operations in the WHERE clause, so we're placing that in the SELECT
		// clause and only making sure it's correct in this place.
		havingConds = " valid_set_modes = " + sm + " "
	}

	// TODO: REDONE THAT SHITCODDING!!!!! ASAP!!!!!
	var beatmapConds string = ""
	if opts.MinAR != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.ar >= %f ", opts.MinAR)
	}
	if opts.MaxAR != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.ar <= %f ", opts.MaxAR)
	}
	if opts.MinOD != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.od >= %f ", opts.MinOD)
	}
	if opts.MaxOD != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.od <= %f ", opts.MaxOD)
	}
	if opts.MinCS != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.cs >= %f ", opts.MinCS)
	}
	if opts.MaxCS != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.cs <= %f ", opts.MaxCS)
	}
	if opts.MinHP != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.hp >= %f ", opts.MinHP)
	}
	if opts.MaxHP != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.hp <= %f ", opts.MaxHP)
	}
	if opts.MinDifficultyRating != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.difficulty_rating >= %f ", opts.MinDifficultyRating)
	}
	if opts.MaxDifficultyRating != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.difficulty_rating <= %f ", opts.MaxDifficultyRating)
	}
	if opts.MinTotalLength != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.total_length >= %v ", opts.MinTotalLength)
	}
	if opts.MaxTotalLength != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.total_length <= %v ", opts.MaxTotalLength)
	}
	if opts.MinBPM != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.bpm >= %f ", opts.MinBPM)
	}
	if opts.MaxBPM != -1 {
		beatmapConds += fmt.Sprintf(" AND beatmaps.bpm <= %f ", opts.MaxBPM)
	}

	if beatmapConds != "" {
		beatmapConds = " AND EXISTS (SELECT 1 FROM beatmaps WHERE beatmaps.parent_set_id = sets.id " + beatmapConds + ") "
	}

	// Limit user amount for beatmap asking
	if opts.Amount > 100 {
		opts.Amount = 100
	}

	sets := make([]SetChimu, 0, opts.Amount)
	setIDs := make([]int, 0, opts.Amount)
	// setMap is used when a query is given to make sure the results are kept in the correct
	// order given by sphinx.
	setMap := make(map[int]int, opts.Amount)
	// if Sphinx is used, limit will be cleared so that it's not used for the mysql query
	limit := fmt.Sprintf(" LIMIT %d, %d ", opts.Offset, opts.Amount)

	if opts.Query != "" {
		setIDsQuery := "SELECT id, set_modes & " + sm + " AS valid_set_modes FROM cg WHERE "

		// add filters to query
		// Yes. I know. Prepared statements. But Sphinx doesn't like them, so
		// bummer.
		setIDsQuery += "MATCH('" + mysqlStringReplacer.Replace(opts.Query) + "') "
		if whereConds != "" {
			setIDsQuery += "AND " + whereConds
		}
		if havingConds != "" {
			setIDsQuery += " AND " + havingConds
		}
		setIDsQuery = strings.ReplaceAll(setIDsQuery, "sets.", "")

		// set limit
		setIDsQuery += " ORDER BY WEIGHT() DESC, id DESC " + limit + " OPTION ranker=sph04, max_matches=20000 "
		limit = ""

		// fetch rows
		rows, err := searchDB.Query(setIDsQuery)
		if err != nil {
			return nil, err
		}

		// contains IDs of the sets we will retrieve
		for rows.Next() {
			var id int
			err = rows.Scan(&id, new(int))
			if err != nil {
				return nil, err
			}
			setIDs = append(setIDs, id)
			sets = sets[:len(sets)+1]
			setMap[id] = len(sets) - 1
		}

		// short path: there are no sets
		if len(sets) == 0 {
			return []SetChimu{}, nil
		}

		whereConds = "sets.id IN (" + sIntCommaSeparated(setIDs) + ")"
		havingConds = ""
	}

	if whereConds != "" {
		whereConds = "WHERE " + whereConds
	}
	if beatmapConds != "" && whereConds == "" {
		beatmapConds = "WHERE " + strings.Replace(beatmapConds, " AND ", " ", 1)
	}
	if havingConds != "" {
		havingConds = " HAVING " + havingConds
	}
	setsQuery := "SELECT " + setFieldsWithRow + ", sets.set_modes & " + sm + " AS valid_set_modes FROM sets " +
		whereConds + beatmapConds + havingConds + " ORDER BY last_update DESC " + limit
	rows, err := db.Query(setsQuery)

	if err != nil {
		return nil, err
	}

	// find all beatmaps, but leave children aside for the moment.
	for rows.Next() {
		var s SetChimu
		err = rows.Scan(
			&s.ID, &s.RankedStatus, &s.ApprovedDate, &s.LastUpdate, &s.LastChecked,
			&s.Artist, &s.Title, &s.Creator, &s.Source, &s.Tags, &s.HasVideo, &s.Genre,
			&s.Language, &s.Favourites, new(int),
		)
		if err != nil {
			return nil, err
		}
		// we get the position we should place s in from the setMap, this way we
		// keep the order of results as sphinx prefers.
		pos, ok := setMap[s.ID]
		if ok {
			sets[pos] = s
		} else {
			sets = append(sets, s)
			setIDs = append(setIDs, s.ID)
			setMap[s.ID] = len(sets) - 1
		}
	}

	if len(sets) == 0 {
		return sets, nil
	}

	rows, err = db.Query(
		"SELECT "+beatmapFields+" FROM beatmaps WHERE parent_set_id IN ("+
			inClause(len(setIDs))+")",
		sIntToSInterface(setIDs)...,
	)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var b BeatmapChimu
		err = rows.Scan(
			&b.ID, &b.ParentSetId, &b.DiffName, &b.FileMD5, &b.Mode, &b.BPM,
			&b.AR, &b.OD, &b.CS, &b.HP, &b.TotalLength, &b.HitLength,
			&b.Playcount, &b.Passcount, &b.MaxCombo, &b.DifficultyRating,
		)
		if err != nil {
			return nil, err
		}

		b.DownloadPath = fmt.Sprintf("/d/%d", b.ParentSetId)
		parentSet, ok := setMap[b.ParentSetId]
		b.OsuFile = fmt.Sprintf("%s - %s (%s) [%s].osu", sets[parentSet].Artist, sets[parentSet].Title, sets[parentSet].Creator, b.DiffName)
		if !ok {
			continue
		}
		sets[parentSet].ChildrenBeatmaps = append(sets[parentSet].ChildrenBeatmaps, b)
	}

	return sets, nil
}
