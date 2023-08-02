package scrapers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const FOUR_REGEX_FINDPOST = `<div id="[^"]+" class="post reply".+?<\/div>(<div class.+?<\/div>)(.+?<\/blockquote><\/div>)`
const FOUR_REGEX_POSTINFO = `class="name">([^<]+).+?data-utc="([^"]+)">([^<]+).+?href="([^"]+)".+?Reply to this post">([^<]+)`
const FOUR_REGEX_POST_TRIM = `.*<blockquote[^>]+>(.+?)</blockquote>`
const FOUR_REGEX_POST_QLINK = `<a href="([^"]+)" class="quotelink">&gt;&gt;([^<]+)<\/a>`
const FOUR_REGEX_POST_QUOTE = `<span class="quote">&gt;([^<]+)</span>`

var FOUR_BOARDS_SFW = []string{
	"a", "c", "g", "k", "m", "o", "p", "v", "vg", "vm", "vmg", "vr", "vrpg",
	"vst", "w", "vip", "qa", "3", "adv", "an", "biz", "cgl", "ck", "co", "diy",
	"fa", "fit", "gd", "his", "int", "jp", "lit", "mu", "n", "news", "out",
	"po", "pw", "qst", "sci", "sp", "tg", "toy", "trv", "tv", "wsg", "wsr",
	"x", "xs"}

var FOUR_BOARDS_NSFW = []string{
	"lgbt", "mlp", "vp", "vt", "b", "r9k", "pol", "bant", "s4s"}

type FourPost struct {
	PosterName string
	DateUTC int64
	DateTime string
	Link string
	PostNumber int64
	Message string
}

var fourstate struct {
	sync.Mutex
	Last int64
}

func fourPostParse(html [][]byte, board string) (FourPost, error) {
	post := FourPost{}
	re := regexp.MustCompile(FOUR_REGEX_POSTINFO)
	reg := re.FindSubmatch(html[1])
	if reg == nil {
		return post, errors.New("failed parsing post info")
	}
	var err error
	post.PosterName = string(reg[1])
	post.DateUTC, err = strconv.ParseInt(string(reg[2]), 10, 64)
	if err != nil {
		return post, err
	}
	post.DateTime = string(reg[3])
	post.Link = "https://boards.4chan.org/" + board + "/" + string(reg[4])
	post.PostNumber, err = strconv.ParseInt(string(reg[5]), 10, 64)
	if err != nil {
		return post, err
	}

	re = regexp.MustCompile(FOUR_REGEX_POST_TRIM)
	reg2 := re.ReplaceAll(html[2], []byte("$1"))
	re = regexp.MustCompile(FOUR_REGEX_POST_QLINK)
	reg2 = re.ReplaceAll(reg2, []byte("[>>$2](https://boards.4chan.org$1)"))
	re = regexp.MustCompile(FOUR_REGEX_POST_QUOTE)
	reg2 = re.ReplaceAll(reg2, []byte("> $1"))
	re = regexp.MustCompile("<br>")
	reg2 = re.ReplaceAll(reg2, []byte("\n"))
	re = regexp.MustCompile("<[^>]+>")
	reg2 = re.ReplaceAll(reg2, []byte(""))
	re = regexp.MustCompile("&#039;")
	reg2 = re.ReplaceAll(reg2, []byte("'"))
	re = regexp.MustCompile("&quot;")
	reg2 = re.ReplaceAll(reg2, []byte("\""))

	post.Message = string(reg2)

	return post, nil
}

func FourPostNewest(board string) (FourPost, error) {
	fourstate.Lock()
	if (time.Now().Unix() - fourstate.Last < CRAWL_RATE) {
		fourstate.Unlock()
		return FourPost{}, errors.New("exceeding crawl rate")
	}
	fourstate.Last = time.Now().Unix()
	fourstate.Unlock()
	client := http.Client{}
	page, err := EasyRequest(
		&client, "https://boards.4chan.org/" + board + "/2")
	if err != nil {
		return FourPost{}, err
	}
	re := regexp.MustCompile(FOUR_REGEX_FINDPOST)
	reg := re.FindSubmatch(page)
	if reg == nil {
		return FourPost{}, errors.New("failed finding post")
	}
	post, err := fourPostParse(reg, board)
	if err != nil {
		return FourPost{}, err
	}

	return post, nil
}
