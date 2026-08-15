package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	token            = ""
	audioLang        = flag.String("audio-lang", "ja-JP", "Audio language(s), comma-separated for multiple (e.g. \"ja-JP,en-US\"). First is the default track")
	subtitlesLang    = flag.String("subs-lang", "en-US", "Subtitle language(s), comma-separated for multiple (e.g. \"en-US,es-419\"). First is the default track")
	videoQuality     = flag.String("video-quality", "1080p", "Video quality")
	audioQuality     = flag.String("audio-quality", "192k", "Audio quality")
	seasonNumber     = flag.Int("season", 0, "Season number. Not used if an episode link is entered")
	etpRt            = flag.String("etp-rt", "", "The \"etp_rt\" cookie value of your account")
	debug            = flag.Bool("debug-manifest", false, "Log raw episode playback JSON and manifest XML")
	output           = flag.String("output-dir", ".", "Target output folder")
	downloadThrottle = flag.Int("throttle", 60, "Throttling between downloads to avoid tripping resource limits. Only used when not specifying a specific episode")
	dryRun           = flag.Bool("dry-run", false, "Act like you will download all of a series but doesn't actually query. Will force throttle to be <=1 if set.")
	verbosity        = flag.Int("loglevel", 2, "Logging level: (0=Errors, 1=Warnings, 2=Info, 3=Debug, 4=Trace).")
	maxWorkers       = flag.Int("max-workers", 10, "The number of workers allowed for downloading streams.")
	slowDownload     = flag.Int("slow-download", 0, "The number of seconds for a background worker to wait before starting another task, slows down downloads to reduce network impact.")
)

// parseLangs splits a comma-separated locale list, trimming spaces and dropping
// empties.
func parseLangs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func processUrl(url string) {
	contentType := strings.Split(url, "/")[3]
	contentId := strings.Split(url, "/")[4]
	if len(contentId) < 9 && len(contentId) > 14 {
		Logf(LogLevel_Error, "Invalid URL format: %s\n", url)
		return
	}
	if contentType != "watch" && contentType != "series" {
		Logf(LogLevel_Error, "Invalid URL (must be /watch/ or /series/): %s\n", url)
		return
	}

	audioLangs := parseLangs(*audioLang)
	if len(audioLangs) == 0 {
		audioLangs = []string{"ja-JP"}
	}
	subsLangs := parseLangs(*subtitlesLang)

	// The season/series API endpoints take a single preferred locale; use the
	// primary (first) requested one. All dub versions are still listed per
	// episode, so the other languages remain resolvable.
	primaryAudio := audioLangs[0]
	primarySubs := "en-US"
	if len(subsLangs) > 0 {
		primarySubs = subsLangs[0]
	}

	if contentType == "watch" {
		info := getEpisodeInfo(contentId)
		_, err := downloadEpisode(contentId, info, audioLangs, subsLangs, videoQuality, audioQuality, *output)
		if err != nil {
			Logf(LogLevel_Error, err.Error())
		}
	} else {
		seasons := getSeasons(contentId, primaryAudio, primarySubs)

		if *seasonNumber != 0 {
			var seasonInd int
			var seasonId string
			for index, season := range seasons {
				if season.SeasonNumber == *seasonNumber {
					seasonId = season.ID
					seasonInd = index
					break
				}
			}
			if seasonId == "" {
				Logf(LogLevel_Warning, "This anime has no season %v!\n", *seasonNumber)
				return
			}

			downloadSeasons(videoQuality, audioQuality, primaryAudio, primarySubs, audioLangs, subsLangs, []Season{seasons[seasonInd]}, *output)
		} else {
			downloadSeasons(videoQuality, audioQuality, primaryAudio, primarySubs, audioLangs, subsLangs, seasons, *output)
		}
	}
}

func main() {
	url := flag.String("url", "", "URL of the episode/season to download")
	urlsFile := flag.String("file", "", "Path to a text file with one URL per line")
	flag.Parse()

	if *url == "" && *urlsFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	if fileinfo, err := os.Stat(*output); os.IsNotExist(err) || !fileinfo.IsDir() {
		Logln(LogLevel_Error, "You must specify a valid directory for outputting files.")
		os.Exit(1)
	}

	if *etpRt == "" {
		Logln(LogLevel_Error, "You must specify the \"-etp-rt\" option!\n- Open Crunchyroll on your browser and log in.\n- Open developer tools (Ctrl+Shift+I), go to \"Application\", and then \"Cookies\".\n- The value of the \"ept_rt\" cookie is what you need to input into this option.")
		os.Exit(1)
	}

	if *downloadThrottle < 0 {
		flag.Usage()
		os.Exit(1)
	}

	if *slowDownload < 0 {
		flag.Usage()
		os.Exit(1)
	}

	if *dryRun && *downloadThrottle > 1 {
		*downloadThrottle = 1
	}

	if *verbosity < 0 || *verbosity >= 5 {

	}
	switch *verbosity {
	case 0:
		GlobalLogLevel = LogLevel_Error
	case 1:
		GlobalLogLevel = LogLevel_Warning
	case 2:
		GlobalLogLevel = LogLevel_Info
	case 3:
		GlobalLogLevel = LogLevel_Debug
	case 4:
		GlobalLogLevel = LogLevel_Trace
	default:
		flag.Usage()
		os.Exit(1)
	}

	token = GetAccessToken(*etpRt)

	if *urlsFile != "" {
		file, err := os.Open(*urlsFile)
		if err != nil {
			Logf(LogLevel_Error, "Failed to open URLs file: %s\n", err)
			os.Exit(1)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var urls []string
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && strings.HasPrefix(line, "http") {
				urls = append(urls, line)
			}
		}

		Logf(LogLevel_Info, "Found %d URLs to download\n\n", len(urls))
		for i, u := range urls {
			Logf(LogLevel_Info, "=== [%d/%d] %s ===\n", i+1, len(urls), u)
			processUrl(u)
			fmt.Println()
		}
	} else {
		processUrl(*url)
	}
}
