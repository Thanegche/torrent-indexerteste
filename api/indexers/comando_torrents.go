package indexers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const (
	URL         = "https://comando.la/"
	queryFilter = "?s="
)

type Audio string

const (
	AudioPortuguese = "Português"
	AudioEnglish    = "Inglês"
	AudioSpanish    = "Espanhol"
)

type IndexedTorrent struct {
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Details       string  `json:"details"`
	Year          string  `json:"year"`
	Audio         []Audio `json:"audio"`
	MagnetLink    string  `json:"magnet_link"`
}

func HandlerComandoIndexer(w http.ResponseWriter, r *http.Request) {
	// supported query params: q, season, episode
	q := r.URL.Query().Get("q")
	if q == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "param q is required"})
		return
	}

	// URL encode query param
	q = url.QueryEscape(q)
	url := URL + queryFilter + q

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

	var indexedTorrents []IndexedTorrent
	for _, link := range links {
		torrents, err := getTorrents(link)
		if err != nil {
			fmt.Println(err)
			continue
		}
		indexedTorrents = append(indexedTorrents, torrents...)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(indexedTorrents)
}

func getTorrents(link string) ([]IndexedTorrent, error) {
	var indexedTorrents []IndexedTorrent
	resp, err := http.Get(link)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	article := doc.Find("article")
	title := article.Find(".entry-title").Text()
	textContent := article.Find("div.entry-content")
	magnets := textContent.Find("a[href^=\"magnet\"]")
	var magnetLinks []string
	magnets.Each(func(i int, s *goquery.Selection) {
		magnetLink, _ := s.Attr("href")
		magnetLinks = append(magnetLinks, magnetLink)
	})

	var audio []Audio
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
		// Legenda: Português
		// Tamanho: –
		// Qualidade de Áudio: 10
		// Qualidade de Vídeo: 10
		// Duração: 59 Min.
		// Servidor: Torrent
		text := s.Text()

		re := regexp.MustCompile(`Áudio: (.*)`)
		audioMatch := re.FindStringSubmatch(text)
		if len(audioMatch) > 0 {
			langs_raw := strings.Split(audioMatch[1], "|")
			for _, lang := range langs_raw {
				lang = strings.TrimSpace(lang)
				if lang == "Português" {
					audio = append(audio, AudioPortuguese)
				} else if lang == "Inglês" {
					audio = append(audio, AudioEnglish)
				} else if lang == "Espanhol" {
					audio = append(audio, AudioSpanish)
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

	// for each magnet link, create a new indexed torrent
	for _, magnetLink := range magnetLinks {
		releaseTitle := extractReleaseName(magnetLink)
		magnetAudio := []Audio{}
		if strings.Contains(strings.ToLower(releaseTitle), "dual") {
			magnetAudio = append(magnetAudio, AudioPortuguese)
			magnetAudio = append(magnetAudio, audio...)
		} else {
			// filter portuguese audio from list
			for _, lang := range audio {
				if lang != AudioPortuguese {
					magnetAudio = append(magnetAudio, lang)
				}
			}
		}

		// remove duplicates
		magnetAudio = removeDuplicates(magnetAudio)

		indexedTorrents = append(indexedTorrents, IndexedTorrent{
			Title:         releaseTitle,
			OriginalTitle: title,
			Details:       link,
			Year:          year,
			Audio:         magnetAudio,
			MagnetLink:    magnetLink,
		})
	}

	return indexedTorrents, nil
}

func extractReleaseName(magnetLink string) string {
	re := regexp.MustCompile(`dn=(.*?)&`)
	matches := re.FindStringSubmatch(magnetLink)
	if len(matches) > 0 {
		return matches[1]
	}
	return ""
}

func removeDuplicates(elements []Audio) []Audio {
	encountered := map[Audio]bool{}
	result := []Audio{}

	for _, element := range elements {
		if !encountered[element] {
			encountered[element] = true
			result = append(result, element)
		}
	}

	return result
}
