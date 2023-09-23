package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/felipemarinho97/torrent-indexer/schema"
	goscrape "github.com/felipemarinho97/torrent-indexer/scrape"
)

const (
	URL         = "https://comando.la/"
	queryFilter = "?s="
)

var replacer = strings.NewReplacer(
	"janeiro", "01",
	"fevereiro", "02",
	"março", "03",
	"abril", "04",
	"maio", "05",
	"junho", "06",
	"julho", "07",
	"agosto", "08",
	"setembro", "09",
	"outubro", "10",
	"novembro", "11",
	"dezembro", "12",
)

type IndexedTorrent struct {
	Title         string         `json:"title"`
	OriginalTitle string         `json:"original_title"`
	Details       string         `json:"details"`
	Year          string         `json:"year"`
	Audio         []schema.Audio `json:"audio"`
	MagnetLink    string         `json:"magnet_link"`
	Date          time.Time      `json:"date"`
	InfoHash      string         `json:"info_hash"`
	Trackers      []string       `json:"trackers"`
	LeechCount    int            `json:"leech_count"`
	SeedCount     int            `json:"seed_count"`
}

func (i *Indexer) HandlerComandoIndexer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// supported query params: q, season, episode
	q := r.URL.Query().Get("q")

	// URL encode query param
	q = url.QueryEscape(q)
	url := URL
	if q != "" {
		url = fmt.Sprintf("%s%s%s", URL, queryFilter, q)
	}

	fmt.Println("URL:>", url)
	resp, err := http.Get(url)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	var links []string
	doc.Find("article").Each(func(i int, s *goquery.Selection) {
		// get link from h2.entry-title > a
		link, _ := s.Find("h2.entry-title > a").Attr("href")
		links = append(links, link)
	})

	var itChan = make(chan []IndexedTorrent)
	var errChan = make(chan error)
	var indexedTorrents []IndexedTorrent
	for _, link := range links {
		go func(link string) {
			torrents, err := getTorrents(ctx, i, link)
			if err != nil {
				fmt.Println(err)
				errChan <- err
			}
			itChan <- torrents
		}(link)
	}

	for i := 0; i < len(links); i++ {
		select {
		case torrents := <-itChan:
			indexedTorrents = append(indexedTorrents, torrents...)
		case err := <-errChan:
			fmt.Println(err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(indexedTorrents)
}

func getTorrents(ctx context.Context, i *Indexer, link string) ([]IndexedTorrent, error) {
	var indexedTorrents []IndexedTorrent
	doc, err := getDocument(ctx, i, link)
	if err != nil {
		return nil, err
	}

	article := doc.Find("article")
	title := strings.Replace(article.Find(".entry-title").Text(), " - Download", "", -1)
	textContent := article.Find("div.entry-content")
	// div itemprop="datePublished"
	datePublished := strings.TrimSpace(article.Find("div[itemprop=\"datePublished\"]").Text())
	// pattern: 10 de setembro de 2021
	re := regexp.MustCompile(`(\d{2}) de (\w+) de (\d{4})`)
	matches := re.FindStringSubmatch(datePublished)
	var date time.Time
	if len(matches) > 0 {
		day := matches[1]
		month := matches[2]
		year := matches[3]
		datePublished = fmt.Sprintf("%s-%s-%s", year, replacer.Replace(month), day)
		date, err = time.Parse("2006-01-02", datePublished)
		if err != nil {
			return nil, err
		}
	}
	magnets := textContent.Find("a[href^=\"magnet\"]")
	var magnetLinks []string
	magnets.Each(func(i int, s *goquery.Selection) {
		magnetLink, _ := s.Attr("href")
		magnetLinks = append(magnetLinks, magnetLink)
	})

	var audio []schema.Audio
	var year string
	article.Find("div.entry-content > p").Each(func(i int, s *goquery.Selection) {
		// pattern:
		// Título Traduzido: Fundação
		// Título Original: Foundation
		// IMDb: 7,5
		// Ano de Lançamento: 2023
		// Gênero: Ação | Aventura | Ficção
		// Formato: MKV
		// Qualidade: WEB-DL
		// Áudio: Português | Inglês
		// Idioma: Português | Inglês
		// Legenda: Português
		// Tamanho: –
		// Qualidade de Áudio: 10
		// Qualidade de Vídeo: 10
		// Duração: 59 Min.
		// Servidor: Torrent
		text := s.Text()

		//re := regexp.MustCompile(`Áudio: (.*)`)
		re := regexp.MustCompile(`(Áudio|Idioma): (.*)`)
		audioMatch := re.FindStringSubmatch(text)
		if len(audioMatch) > 0 {
			sep := getSeparator(audioMatch[2])
			langs_raw := strings.Split(audioMatch[2], sep)
			for _, lang := range langs_raw {
				lang = strings.TrimSpace(lang)
				a := schema.GetAudioFromString(lang)
				if a != nil {
					audio = append(audio, *a)
				} else {
					fmt.Println("unknown language:", lang)
				}
			}
		}

		re = regexp.MustCompile(`Lançamento: (.*)`)
		yearMatch := re.FindStringSubmatch(text)
		if len(yearMatch) > 0 {
			year = yearMatch[1]
		}

		// if year is empty, try to get it from title
		if year == "" {
			re = regexp.MustCompile(`\((\d{4})\)`)
			yearMatch := re.FindStringSubmatch(title)
			if len(yearMatch) > 0 {
				year = yearMatch[1]
			}
		}
	})

	var chanIndexedTorrent = make(chan IndexedTorrent)

	// for each magnet link, create a new indexed torrent
	for _, magnetLink := range magnetLinks {
		go func(magnetLink string) {
			releaseTitle := extractReleaseName(magnetLink)
			magnetAudio := []schema.Audio{}
			if strings.Contains(strings.ToLower(releaseTitle), "dual") {
				magnetAudio = append(magnetAudio, audio...)
			} else if len(audio) > 1 {
				// remove portuguese audio, and append to magnetAudio
				for _, a := range audio {
					if a != schema.AudioPortuguese {
						magnetAudio = append(magnetAudio, a)
					}
				}
			} else {
				magnetAudio = append(magnetAudio, audio...)
			}
			// decode url encoded title
			releaseTitle, _ = url.QueryUnescape(releaseTitle)

			infoHash := extractInfoHash(magnetLink)
			trackers := extractTrackers(magnetLink)
			peer, seed, err := goscrape.GetLeechsAndSeeds(ctx, i.redis, infoHash, trackers)
			if err != nil {
				fmt.Println(err)
			}

			title := processTitle(title, magnetAudio)

			it := IndexedTorrent{
				Title:         releaseTitle,
				OriginalTitle: title,
				Details:       link,
				Year:          year,
				Audio:         magnetAudio,
				MagnetLink:    magnetLink,
				Date:          date,
				InfoHash:      infoHash,
				Trackers:      trackers,
				LeechCount:    peer,
				SeedCount:     seed,
			}
			chanIndexedTorrent <- it
		}(magnetLink)
	}

	for i := 0; i < len(magnetLinks); i++ {
		it := <-chanIndexedTorrent
		indexedTorrents = append(indexedTorrents, it)
	}

	return indexedTorrents, nil
}

func processTitle(title string, a []schema.Audio) string {
	// remove ' - Donwload' from title
	title = strings.Replace(title, " - Download", "", -1)

	// remove 'comando.la' from title
	title = strings.Replace(title, "comando.la", "", -1)

	// add audio ISO 639-2 code to title between ()
	if len(a) > 0 {
		audio := []string{}
		for _, lang := range a {
			audio = append(audio, lang.String())
		}
		title = fmt.Sprintf("%s (%s)", title, strings.Join(audio, ", "))
	}

	return title
}

func getSeparator(s string) string {
	if strings.Contains(s, "|") {
		return "|"
	} else if strings.Contains(s, ",") {
		return ","
	}
	return " "
}

func getDocument(ctx context.Context, i *Indexer, link string) (*goquery.Document, error) {
	// try to get from redis first
	docCache, err := i.redis.Get(ctx, link)
	if err == nil {
		return goquery.NewDocumentFromReader(ioutil.NopCloser(bytes.NewReader(docCache)))
	}

	resp, err := http.Get(link)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// set cache
	err = i.redis.Set(ctx, link, body)
	if err != nil {
		fmt.Println(err)
	}

	doc, err := goquery.NewDocumentFromReader(ioutil.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func extractReleaseName(magnetLink string) string {
	re := regexp.MustCompile(`dn=(.*?)&`)
	matches := re.FindStringSubmatch(magnetLink)
	if len(matches) > 0 {
		return matches[1]
	}
	return ""
}

func extractInfoHash(magnetLink string) string {
	re := regexp.MustCompile(`btih:(.*?)&`)
	matches := re.FindStringSubmatch(magnetLink)
	if len(matches) > 0 {
		return matches[1]
	}
	return ""
}

func extractTrackers(magnetLink string) []string {
	re := regexp.MustCompile(`tr=(.*?)&`)
	matches := re.FindAllStringSubmatch(magnetLink, -1)
	var trackers []string
	for _, match := range matches {
		// url decode
		tracker, _ := url.QueryUnescape(match[1])
		trackers = append(trackers, tracker)
	}
	return trackers
}
