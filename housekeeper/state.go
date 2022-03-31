package housekeeper

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CachedBeatmap represents a beatmap that is held in the cache of CheeseGull.
type CachedBeatmap struct {
	ID          int
	NoVideo     bool
	LastUpdate  time.Time
	DataFolders []string

	lastRequested time.Time

	fileSize     uint64
	isDownloaded bool
	mtx          sync.RWMutex
	waitGroup    sync.WaitGroup
}

func (c *CachedBeatmap) UpdateFolders(folders string) bool {
	var splittedPaths = strings.Split(folders, ",")
	c.DataFolders = splittedPaths
	return true
}

// File opens the File of the beatmap from the filesystem.
func (c *CachedBeatmap) File() (*os.File, error) {
	if len(c.DataFolders) < 1 {
		return nil, os.ErrNotExist
	}

	for _, path := range c.DataFolders {
		if _, err := os.Stat(path + c.fileName()); errors.Is(err, os.ErrNotExist) {
			continue
		}
		// file exists...
		return os.Open(path + c.fileName())
	}
	return os.Open(c.DataFolders[len(c.DataFolders)-1] + c.fileName())
}

// CreateFile creates the File of the beatmap in the filesystem, and returns it
// in write mode.
func (c *CachedBeatmap) CreateFile() (*os.File, error) {
	if len(c.DataFolders) < 1 {
		return nil, os.ErrInvalid
	}

	return os.Create(c.DataFolders[len(c.DataFolders)-1] + c.fileName())
}

func (c *CachedBeatmap) fileName() string {
	// n := ""
	// if c.NoVideo {
	// 	n = "n"
	// }
	return strconv.Itoa(c.ID) + ".osz"
}

// IsDownloaded checks whether the beatmap has been downloaded.
func (c *CachedBeatmap) IsDownloaded() bool {
	c.mtx.RLock()
	i := c.isDownloaded
	c.mtx.RUnlock()
	return i
}

// GetLastAttempt Get Last Attempt time for re-download in some cases ;d
func (c *CachedBeatmap) GetLastAttempt() int {
	return int(c.lastRequested.Unix())
}

// FileSize returns the FileSize of c.
func (c *CachedBeatmap) FileSize() uint64 {
	c.mtx.RLock()
	i := c.fileSize
	c.mtx.RUnlock()
	return i
}

// MustBeDownloaded will check whether the beatmap is downloaded.
// If it is not, it will wait for it to become downloaded.
func (c *CachedBeatmap) MustBeDownloaded() {
	if c.IsDownloaded() {
		return
	}
	c.waitGroup.Wait()
}

// DownloadCompleted must be called once the beatmap has finished downloading.
func (c *CachedBeatmap) DownloadCompleted(fileSize uint64, parentHouse *House) {
	c.mtx.Lock()
	c.fileSize = fileSize
	c.isDownloaded = true
	c.mtx.Unlock()
	c.waitGroup.Done()
	parentHouse.scheduleCleanup()
}

// NotDownloaded must be called when file is fucking empty!
func (c *CachedBeatmap) NotDownloaded(parentHouse *House) {
	for _, path := range c.DataFolders {
		if _, err := os.Stat(path + c.fileName()); errors.Is(err, os.ErrNotExist) {
			continue
		}
		// file exists...
		var f, err = os.Open(path + c.fileName())
		stat, err := f.Stat()
		if err == nil && stat.Size() < 100 {
			os.Remove(path + c.fileName())
		}
		defer f.Close()
	}

	c.mtx.Lock()
	c.fileSize = 0
	c.isDownloaded = false
	c.mtx.Unlock()
	c.waitGroup.Done()
	parentHouse.scheduleCleanup()
}

// SetLastRequested changes the last requested time.
func (c *CachedBeatmap) SetLastRequested(t time.Time) {
	c.mtx.Lock()
	c.lastRequested = t
	c.mtx.Unlock()
}

func (c *CachedBeatmap) String() string {
	return fmt.Sprintf("{ID: %d NoVideo: %t LastUpdate: %v}", c.ID, c.NoVideo, c.LastUpdate)
}

// AcquireBeatmap attempts to add a new CachedBeatmap to the state.
// In order to add a new CachedBeatmap to the state, one must not already exist
// in the state with the same ID, NoVideo and LastUpdate. In case one is already
// found, this is returned, alongside with false. If LastUpdate is newer than
// that of the beatmap stored in the state, then the beatmap in the state's
// downloaded status is switched back to false and the LastUpdate is changed.
// true is also returned, indicating that the caller now has the burden of
// downloading the beatmap.
//
// In the case the cachedbeatmap has not been stored in the state, then
// it is added to the state and, like the case where LastUpdated has been
// changed, true is returned, indicating that the caller must now download the
// beatmap.
//
// If you're confused attempting to read this, let me give you an example:
//
//   A: Yo, is this beatmap cached?
//   B: Yes, yes it is! Here you go with the information about it. No need to do
//      anything else.
//      ----
//   A: Yo, got this beatmap updated 2 hours ago. Have you got it cached?
//   B: Ah, I'm afraid that I only have the version updated 10 hours ago.
//      Mind downloading the updated version for me?
//      ----
//   A: Yo, is this beatmap cached?
//   B: Nope, I didn't know it existed before you told me. I've recorded its
//      info now, but jokes on you, you now have to actually download it.
//      Chop chop!
func (h *House) AcquireBeatmap(c *CachedBeatmap) (*CachedBeatmap, bool) {
	if c == nil {
		return nil, false
	}

	h.StateMutex.Lock()
	for _, b := range h.State {
		// if the id or novideo is different, then all is good and we
		// can proceed with the next element.
		if b.ID != c.ID || b.NoVideo != c.NoVideo {
			continue
		}
		// unlocking because in either branch, we will return.
		h.StateMutex.Unlock()

		b.mtx.Lock()
		b.DataFolders = c.DataFolders
		// if c is not newer than b, then just return.
		if !b.LastUpdate.Before(c.LastUpdate) {
			b.mtx.Unlock()
			return b, false
		}

		b.LastUpdate = c.LastUpdate
		b.mtx.Unlock()
		b.waitGroup.Add(1)
		return b, true
	}

	// c was not present in our state: we need to add it.

	// we need to recreate the CachedBeatmap: this way we can be sure the zero
	// is set for the unexported fields.
	n := &CachedBeatmap{
		ID:          c.ID,
		NoVideo:     c.NoVideo,
		LastUpdate:  c.LastUpdate,
		DataFolders: h.DataFolders,
	}
	h.State = append(h.State, n)
	h.StateMutex.Unlock()

	n.waitGroup.Add(1)
	return n, true
}
