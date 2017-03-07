package main

import (
	"flag"
	"fmt"
	vlc "github.com/kestein/go-vlc"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const videoPath string = "/home/kestein/Videos"
const screenshotPath = "/home/kestein/Pictures/screenshots"
const referer string = "Referer"
const secToMilli int64 = 1000
const pageStart string = `
<html>
	<body>
		<ul>
`
const content string = `
			{{block "links" .}}
				{{range .}}
					<li><a href=/{{.Action}}/{{.Url}}>{{.Filename}}</a></li>
				{{end}}
			{{end}}
`
const pageEnd string = `
		</ul>
	</body>
</html>
`

type videoLine struct {
	Action, Url, Filename string
}

type cartoon struct {
	inst    *vlc.Instance
	player  *vlc.Player
	media   *vlc.Media
	vidLen  int64
	playing bool
}

type videoLineList []videoLine

func (a videoLineList) Len() int           { return len(a) }
func (a videoLineList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a videoLineList) Less(i, j int) bool { return a[i].Url < a[j].Url }

func list(w http.ResponseWriter, req *http.Request) {
	localURL := req.RequestURI[6:] // Take out the '/list'
	// ls the directory
	endDir, err := url.QueryUnescape(localURL)
	if err != nil {
		log.Fatal("Unable to parse video path %s", err)
	}
	fullPath, err := filepath.Abs(fmt.Sprintf("%s/%s", videoPath, endDir))
	if err != nil {
		log.Fatal("%s not a good path", fullPath)
	}
	// Check the paths to ensure that only videos are considered
	if len(fullPath) >= len(videoPath) {
		if fullPath[0:len(videoPath)] != videoPath {
			http.Error(w, "Not in the video path", http.StatusUnauthorized)
		}
	}
	dir, err := os.Open(fullPath)
	if err != nil {
		log.Fatal("Unable to open video path %s", err)
	}
	files, err := dir.Readdir(0)
	if err != nil {
		log.Fatal("Unable to read dir %s", err)
	}
	// Write the HTML. Sort out by directories and files
	subDirs := []videoLine{}
	orphanFiles := []videoLine{}
	for i := 0; i < len(files); i++ {
		// For printing
		fileName := files[i].Name()
		// For RESTing
		vidURL := ""
		if len(req.URL.String()) > 0 {
			vidURL = fmt.Sprintf("%s/%s", req.URL.String(), url.QueryEscape(fileName))
		} else {
			vidURL = fmt.Sprintf("%s", url.QueryEscape(fileName))
		}
		if files[i].IsDir() {
			url := videoLine{"list", vidURL, fileName}
			subDirs = append(subDirs, url)
		} else {
			url := videoLine{"play", vidURL, fileName}
			orphanFiles = append(orphanFiles, url)
		}
	}
	// Write HTML to the reponse
	sort.Sort(videoLineList(subDirs))
	// Make the '..' directory at the top
	prev := strings.Split(req.URL.String(), "/")
	back := ""
	if len(prev) > 1 {
		back = strings.Join(prev[:len(prev)-1], "/")
	}
	previousLink := []videoLine{videoLine{"list", back, ".."}}
	subDirs = append(previousLink, subDirs...)
	sort.Sort(videoLineList(orphanFiles))
	// Write the HTML
	io.WriteString(w, pageStart)
	t := template.Must(template.New("content").Parse(content))
	err = t.Execute(w, subDirs)
	if err != nil {
		panic(err)
	}
	err = t.Execute(w, orphanFiles)
	if err != nil {
		panic(err)
	}
	io.WriteString(w, pageEnd)
}

func play(state *cartoon, w http.ResponseWriter, req *http.Request) {
	var media *vlc.Media
	// var evt *vlc.EventManager
	var err error

	// Obtain the name of the video to view
	rawVideo := req.RequestURI[6:] // Take out the '/play'
	video, err := url.QueryUnescape(rawVideo)
	if err != nil {
		fmt.Println("WRONG")
		return
	}
	// Make VLC instance
	if state.inst, err = vlc.New([]string{}); err != nil {
		fmt.Println(err)
		return
	}
	// Open the video file
	if media, err = state.inst.OpenMediaFile(fmt.Sprintf("%s/%s", videoPath, video)); err != nil {
		fmt.Println(err)
		return
	}
	state.media = media
	// Create the media player
	if state.player, err = media.NewPlayer(); err != nil {
		fmt.Println(err)
		return
	}
	// Initialize player state
	state.player.SetVolume(25)
	//state.player.SetFullscreen(true)

	// Make the page to control the video
	state.player.Play()
	// Wait for the player to start playing
	for {
		t, err := state.player.Length()
		if err != nil {
			panic(err)
		}
		if t > 0 {
			break
		}
	}
	vidLen, err := state.player.Length()
	state.vidLen = vidLen
	vidLen = vidLen / secToMilli
	state.playing = true
	t := template.Must(template.New("player.html").ParseFiles("./player.html"))
	vals := map[string]int64{
		"vol":  25,
		"secs": vidLen,
	}
	t.Execute(w, vals)
	//io.WriteString(w, playerPage)
}

func stop(state *cartoon, w *http.ResponseWriter, req *http.Request) {
	if state.player == nil {
		return
	}
	state.player.Stop()
	state.playing = false
	closePlayer(state)
}

func closePlayer(state *cartoon) {
	if state.player == nil {
		return
	}
	state.player.Release()
	state.inst.Release()
	state.media.Release()
	state.player = nil
	state.inst = nil
	state.media = nil
	state.vidLen = 0
}

func pausePlay(state *cartoon, w *http.ResponseWriter) {
	if state.player == nil {
		return
	}
	// Replay the video
	if !state.player.WillPlay() {
		replay(state)
	} else {
		state.player.TogglePause(state.playing)
		state.playing = !state.playing
	}
	curTime := vidTime(state)
	io.WriteString(*w, fmt.Sprintf("%d", curTime/secToMilli))
}

func replay(state *cartoon) {
	var e error
	state.player.Release()
	state.player, e = state.media.NewPlayer()
	if e != nil {
		panic(e)
	}
	// re set the volume
	state.player.Play()
	state.playing = true
}

func rewind(state *cartoon) {
	if state.player == nil {
		return
	}
	var secsRewound int64 = 10
	curTime := vidTime(state)
	rTime := curTime - (secsRewound * secToMilli)
	if rTime < 0 {
		state.player.SetTime(0)
	} else {
		state.player.SetTime(rTime)
	}
}

/* Returns the time of the video in milliseconds */
func vidTime(state *cartoon) int64 {
	curTime, err := state.player.Time()
	if err != nil {
		panic(err)
	}
	return curTime
}

func setVolume(state *cartoon, toVol int) {
	if state.player == nil {
		return
	}
	state.player.SetVolume(toVol)
}

func setTime(state *cartoon, seek int64, w *http.ResponseWriter) {
	if state.player == nil {
		return
	}
	if !state.player.WillPlay() {
		replay(state)
	}
	state.player.SetTime(seek * secToMilli)
	curTime := vidTime(state)
	io.WriteString(*w, fmt.Sprintf("%d", curTime/secToMilli))
}

func screenshot(state *cartoon) {
	if state.player == nil {
		return
	}
	state.player.TakeSnapshot(screenshotPath, 0, 0, 0)
}

func main() {
	port := flag.String("p", "8100", "port to serve on")
	directory := flag.String("d", ".", "the directory of static file to host")
	flag.Parse()

	state := cartoon{
		inst:    nil,
		player:  nil,
		media:   nil,
		playing: false,
		vidLen:  0,
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "list", 301)
	})
	http.Handle("/list/", http.StripPrefix("/list/", http.HandlerFunc(list)))
	http.Handle("/play/", http.StripPrefix("/play/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			play(&state, w, req)
		})))
	http.Handle("/playback/", http.StripPrefix("/playback/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			pausePlay(&state, &w)
		})))
	http.Handle("/rewind/", http.StripPrefix("/rewind/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			rewind(&state)
		})))
	http.Handle("/stop/", http.StripPrefix("/stop/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			stop(&state, &w, req)
		})))
	http.Handle("/screenshot/", http.StripPrefix("/screenshot/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			screenshot(&state)
		})))
	// If this API endpoint is called "seek" it breaks if you try to seek to 0.
	// WTF
	http.Handle("/time/", http.StripPrefix("/time/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			seekPos, err := strconv.ParseInt(req.URL.String(), 10, 64)
			if err != nil {
				fmt.Println("NaN")
				return
			}
			setTime(&state, seekPos, &w)
		})))
	http.Handle("/volume/", http.StripPrefix("/volume/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			toVol, err := strconv.ParseInt(req.URL.String(), 10, 64)
			if err != nil {
				fmt.Println("NaN")
				return
			}
			setVolume(&state, int(toVol))
		})))
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})

	log.Printf("Serving %s on HTTP port: %s\n", *directory, *port)
	log.Fatal(http.ListenAndServe("localhost:"+*port, nil))
}
