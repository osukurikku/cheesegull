// Package downloader implements downloading from the osu! website, through,
// well, mostly scraping and dirty hacks.
package downloader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
)

var downloadHostName string
var bmsOsuKey string

// SetHostName describes from which host i want download
func SetHostName(host string) {
	downloadHostName = host
}

// SetBmsOsuKey describes special_key for bms_osu.kotworks.cyou
func SetBmsOsuKey(key string) {
	bmsOsuKey = key
}

// LogIn logs in into an osu! account and returns a Client.
func LogIn(username, password string) (*Client, error) {
	j, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return nil, err
	}
	c := &http.Client{
		Jar: j,
	}
	vals := url.Values{}
	vals.Add("redirect", "/")
	vals.Add("sid", "")
	vals.Add("username", username)
	vals.Add("password", password)
	vals.Add("autologin", "on")
	vals.Add("login", "login")
	_, err = c.PostForm("https://old.ppy.sh/forum/ucp.php?mode=login", vals)
	if err != nil {
		return nil, err
	}

	return (*Client)(c), nil
}

// Client is a wrapper around an http.Client which can fetch beatmaps from the
// osu! website.
type Client http.Client

// HasVideo checks whether a beatmap has a video.
func (c *Client) HasVideo(setID int) (bool, error) {
	h := (*http.Client)(c)

	page, err := h.Get(fmt.Sprintf("https://old.ppy.sh/s/%d", setID))
	if err != nil {
		return false, err
	}
	defer page.Body.Close()
	body, err := ioutil.ReadAll(page.Body)
	if err != nil {
		return false, err
	}
	return bytes.Contains(body, []byte(fmt.Sprintf(`href="/d/%dn"`, setID))), nil
}

// Download downloads a beatmap from the osu! website. noVideo specifies whether
// we should request the beatmap to not have the video.
func (c *Client) Download(setID int, noVideo bool) (io.ReadCloser, error) {
	// suffix := ""
	// if noVideo {
	// 	suffix = "n"
	// }
	return c.getReader(strconv.Itoa(setID))
}

// ErrNoRedirect is returned from Download when we were not redirect, thus
// indicating that the beatmap is unavailable.
var ErrNoRedirect = errors.New("cheesegull/downloader: no redirect happened, beatmap could not be downloaded")

var errNoZip = errors.New("cheesegull/downloader: file is not a zip archive")

const zipMagic = "PK\x03\x04"

func (c *Client) getReader(str string) (io.ReadCloser, error) {
	h := (*http.Client)(c)

	var globalerr error
	/*
		Just some information about hosts:
		Hosts are needed for downloading maps directly from N mirrors.
		We store our maps already in cache folder, so engine will just send that files, BUT
		if we lost something, or our additional crawler not working well, that thing exists...
	*/
	hosts := []string{
		fmt.Sprintf("https://%s/d/", downloadHostName) + "%s?novideo=1",
		"https://storage.ripple.moe/d/%s?novideo=1",
		"https://txy1.sayobot.cn/beatmaps/download/full/%sn?server=null",
	}

	for _, host := range hosts {
		log.Println("[I] Trying download", str, "from", host)
		resp, err := h.Get(fmt.Sprintf(host, str))
		if err != nil {
			log.Println("[I] Download failed", str, "from", host)
			globalerr = err
			continue // skip to next host
		}
		if resp.Request.URL.Host == "old.ppy.sh" {
			resp.Body.Close()
			globalerr = ErrNoRedirect
			continue // skip to next host
		}

		// check that it is a zip file
		first4 := make([]byte, 4)
		_, err = resp.Body.Read(first4)
		if err != nil {
			log.Println("[I] Download failed (can't read 4 bytes)", str, "from", host)
			globalerr = err
			continue // skip to next host
		}
		if string(first4) != zipMagic {
			log.Println("[I] Downloaded file doesn't contain zipMagic", str, "from", host)
			globalerr = errNoZip
			continue // skip to next host
		}

		log.Println("[I] Download complete", str, "from", host)
		return struct {
			io.Reader
			io.Closer
		}{
			io.MultiReader(strings.NewReader(zipMagic), resp.Body),
			resp.Body,
		}, nil
	}

	// By my logic, it should be called when all is shit
	return nil, globalerr
}
