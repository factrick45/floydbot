package scrapers

import (
	"encoding/csv"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DESU_CACHE_LIFETIME = 2419200 // Four more weeks

// TODO: replace arg spam with this
type desuData struct {
	Board string
	Term string
	Timestamp int64
	Urls []string
}

var desustate struct {
	sync.Mutex
	Scraping bool
}

func desuCacheLoad(board string, term string) (int64, []string, error) {
	bytes, err := os.ReadFile("cache/desu_" + board + "_" + term + ".csv")
	if err != nil {
		log.Println("error reading cache file:", err)
		return 0, nil, err
	}

	r := csv.NewReader(strings.NewReader(string(bytes)))

	// Get timestamp
	record, err := r.Read()
	if err != nil {
		log.Println("error parsing cache timestamp:", err)
		return 0, nil, err
	}
	// Must be int64!
	timestamp, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		log.Println("error parsing cache timestamp:", err)
		return 0, nil, err
	}

	// Get URLs
	urls := []string{}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("error parsing cache URLs:", err)
			return 0, nil, err
		}
		urls = append(urls, record[0])
	}

	return timestamp, urls, nil
}

func desuCacheSave(board string, term string, timestamp int64, urls []string) {
	f, err := os.OpenFile(
		"cache/desu_" + board + "_" + term + ".csv",
		os.O_RDWR | os.O_CREATE, 0644)
	if err != nil {
		log.Println("error opening cache file:", err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)

	// Write timestamp
	err = w.Write([]string{strconv.FormatInt(timestamp, 10)})
	if err != nil {
		log.Println("error formatting cache:", err)
		return
	}

	// Write urls
	for _, url := range urls {
		if err = w.Write([]string{url}); err != nil {
			log.Println("error formatting cache:", err)
			return
		}
	}

	w.Flush()
	if err = w.Error(); err != nil {
		log.Println("error writing to cache file:", err)
		return
	}
}

func desuScrapeUpdate(board string, term string) {
	desustate.Lock()
	if desustate.Scraping {
		desustate.Unlock()
		return
	}
	desustate.Scraping = true
	desustate.Unlock()

	log.Println(
		"Cache lifetime for " + board + "_" + term + " expired. Rescraping...")

	defer func() {
		desustate.Lock()
		desustate.Scraping = false
		desustate.Unlock()
	}()

	// Finally the actual scraping
	client := &http.Client{}
	// Get the first page
	req, err := http.NewRequest(
		"GET", "https://desuarchive.org/" + board + "/search/subject/" + term,
		nil)
	if err != nil {
		log.Println(err)
		return
	}
	req.Header.Set("User-Agent", USER_AGENT)
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	if resp.StatusCode != 200 {
		log.Println("error response:", resp.Status)
		resp.Body.Close()
		return
	}
	page0, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Println(err)
		return
	}

	// Extract the URLs of the other pages
	re := regexp.MustCompile(`<li><a href="(https://desuarchive.org/\w+/search/subject/[^/]+/page/[0-9]+/)`)
	reg := re.FindAllSubmatch(page0, -1)
	if reg == nil {
		log.Println("Error parsing Desuarchive: no pages")
		return
	}

	// Request each page
	pagebodies := [][]byte{page0}
	for _, url := range reg {
		time.Sleep(CRAWL_RATE * time.Second)
		req, err = http.NewRequest("GET", string(url[1]), nil)
		if err != nil {
			log.Println(err)
			return
		}
		req.Header.Set("User-Agent", USER_AGENT)
		resp, err = client.Do(req)
		if err != nil {
			log.Println(err)
			return
		}
		if resp.StatusCode != 200 {
			log.Println("error response:", resp.Status)
			resp.Body.Close()
			return
		}
		page, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Println(err)
			return
		}
		pagebodies = append(pagebodies, page)
	}

	// Extract the thread URLs from each page
	threadurls := [][]byte{}
	re = regexp.MustCompile(`<span class="time_wrap"> <time [^<]+</time> </span> <a href="(https://[^#]+)`)
	for _, page := range pagebodies {
		reg = re.FindAllSubmatch(page, -1)
		if reg == nil {
			log.Println("Error parsing Desuarchive: no threads in page")
		}
		for _, url := range reg {
			threadurls = append(threadurls, url[1])
		}
	}

	// Request each thread and extract the image URLs
	imgurls := []string{}
	re = regexp.MustCompile(
		`class="thread_image_box"> <a href="(https://desu[^"]+)`)
	for _, threadurl := range threadurls {
		time.Sleep(CRAWL_RATE * time.Second)
		req, err = http.NewRequest("GET", string(threadurl), nil)
		if err != nil {
			log.Println(err)
			return
		}
		req.Header.Set("User-Agent", USER_AGENT)
		resp, err = client.Do(req)
		if err != nil {
			log.Println(err)
			return
		}
		if resp.StatusCode != 200 {
			log.Println("error response:", resp.Status)
			resp.Body.Close()
			return
		}
		thread, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Println(err)
			return
		}
		reg = re.FindAllSubmatch(thread, -1)
		if reg == nil {
			log.Println("Error parsing Desuarchive: no images in thread")
		}
		for _, url := range reg {
			imgurls = append(imgurls, string(url[1]))
		}
	}

	log.Println("Finished scraping " + board + "_" + term + "!")

	desuCacheSave(board, term, time.Now().Unix(), imgurls)
}

func DesuScrape(board string, term string) (string, bool, error) {
	ctime, curls, err := desuCacheLoad(board, term)
	if err != nil {
		log.Println("error loading cache:", err)
	}
	if (ctime == 0 || time.Now().Unix() - ctime > DESU_CACHE_LIFETIME) {
		go desuScrapeUpdate(board, term)
	}

	if err == nil {
		img := curls[rand.Intn(len(curls))]
		return img, strings.Contains(img, ".webm"), nil
	}
	// The caller can use a nice placeholder image
	return "https://files.catbox.moe/b9jz4v.png", false, err
}
