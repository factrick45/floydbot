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

type DesuData struct {
	Board string
	Term string
	Time int64
	Urls []string
}

var desustate struct {
	sync.Mutex
	Scraping bool
}

func desuCacheLoad(d *DesuData) error {
	bytes, err := os.ReadFile(
		"cache/desu_" + d.Board + "_" + d.Term + ".csv")
	if err != nil {
		log.Println("error reading cache file:", err)
		return err
	}

	r := csv.NewReader(strings.NewReader(string(bytes)))

	// Get timestamp
	record, err := r.Read()
	if err != nil {
		log.Println("error parsing cache timestamp:", err)
		return err
	}
	// Must be int64!
	timestamp, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		log.Println("error parsing cache timestamp:", err)
		return err
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
			return err
		}
		urls = append(urls, record[0])
	}

	d.Time = timestamp
	d.Urls = urls
	return nil
}

func desuCacheSave(d *DesuData) error {
	f, err := os.OpenFile(
		"cache/desu_" + d.Board + "_" + d.Term + ".csv",
		os.O_RDWR | os.O_CREATE, 0644)
	if err != nil {
		log.Println("error opening cache file:", err)
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)

	// Write timestamp
	err = w.Write([]string{strconv.FormatInt(d.Time, 10)})
	if err != nil {
		log.Println("error formatting cache:", err)
		return err
	}

	// Write urls
	for _, url := range d.Urls {
		if err = w.Write([]string{url}); err != nil {
			log.Println("error formatting cache:", err)
			return err
		}
	}

	w.Flush()
	if err = w.Error(); err != nil {
		log.Println("error writing to cache file:", err)
		return err
	}

	return nil
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
	client := http.Client{}
	// Get the first page
	page0, err := EasyRequest(
		&client, "https://desuarchive.org/" + board + "/search/subject/" + term)
	if err != nil {
		log.Println(err)
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
		page, err := EasyRequest(&client, string(url[1]))
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
		thread, err := EasyRequest(&client, string(threadurl))
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

	desuCacheSave(&DesuData{
		Board: board, Term: term, Time: time.Now().Unix(), Urls: imgurls})
}

func DesuScrape(board string, term string) (string, bool, error) {
	d := DesuData{Board: board, Term: term}
	err := desuCacheLoad(&d)
	if err != nil {
		log.Println("error loading cache:", err)
	}
	if (err != nil || time.Now().Unix() - d.Time > DESU_CACHE_LIFETIME) {
		go desuScrapeUpdate(board, term)
	}

	if err == nil {
		img := d.Urls[rand.Intn(len(d.Urls))]
		return img, strings.Contains(img, ".webm"), nil
	}
	// The caller can use a nice placeholder image
	return "https://files.catbox.moe/b9jz4v.png", false, err
}
