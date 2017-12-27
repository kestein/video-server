package main

import (
	"flag"
	"fmt"
	"github.com/YouROK/go-mpv/mpv"
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
const initVolume int = 75
const secToMilli int64 = 1000
const uninitializedSubs int = -1000
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
	player *mpv.Mpv
	video  string
	vidLen int64
	paused bool
	subbed bool
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
			vidURL = fmt.Sprintf("%s/%s", url.QueryEscape(fullPath[len(videoPath)+1:]), url.QueryEscape(fileName))
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
	var err error

	// Obtain the name of the video to view
	rawVideo := req.RequestURI[6:] // Take out the '/play/'
	video, err := url.QueryUnescape(rawVideo)
	if err != nil {
		panic(err)
	}
	// Start the video
	startPlay(state, video)
	state.video = video
	t := template.Must(template.New("player.html").ParseFiles("./player.html"))
	vals := map[string]int64{
		"vol":  int64(75),
		"secs": state.vidLen,
	}
	t.Execute(w, vals)
	//io.WriteString(w, playerPage)
}

func startPlay(state *cartoon, video string) {
	var player *mpv.Mpv = mpv.Create()
	go func() {
		for {
			e := player.WaitEvent(1)
			if e.Event_Id == mpv.EVENT_END_FILE {
				stop(state)
				closePlayer(state)
				break
			}
		}
	}()
	// Options ricing
	player.SetOptionString("fullscreen", "yes")
	player.SetOptionString("screenshot-format", "png")
	player.SetOptionString("screenshot-directory", screenshotPath)
	player.SetOptionString("screenshot-template", "%F-%P-%n")
	player.SetOptionString("screenshot-png-compression", "0")
	player.SetOptionString("screenshot-png-filter", "0")
	player.SetOptionString("screenshot-tag-colorspace", "yes")
	player.SetOptionString("screenshot-high-bit-depth", "yes")
	player.SetOptionString("slang", "eng,en,enUS,en-US")
	player.SetOptionString("alang", "jpn,jp,eng,en,enUS,en-US")
	err := player.Initialize()
	if err != nil {
		fmt.Println("failed to initialize ", err)
		panic("")
	}
	err = player.Command([]string{"loadfile", fmt.Sprintf("%s/%s", videoPath, video)})
	if err != nil {
		fmt.Println("failed to loadfile ", err)
		panic("")
	}
	state.player = player
	state.vidLen = vidLen(state)
	state.paused = false
	state.subbed = true
}

func stop(state *cartoon) {
	if state.player == nil {
		return
	}
	err := state.player.Command([]string{"stop"})
	if err != nil {
		fmt.Println("unable to stop ", err)
		panic("")
	}
	state.paused = true
}

func closePlayer(state *cartoon) {
	if state.player == nil {
		return
	}
	state.player.TerminateDestroy()
	state.player = nil
	state.vidLen = 0
}

func pausePlay(state *cartoon, w *http.ResponseWriter) {
	var err error
	if state.player == nil && len(state.video) > 0 {
		startPlay(state, state.video)
		io.WriteString(*w, fmt.Sprintf("%d", 0))
		return
	}
	if state.paused {
		err = state.player.SetOptionString("pause", "no")
	} else {
		err = state.player.SetOptionString("pause", "yes")
	}
	if err != nil {
		fmt.Println("problem toggling play ", err)
		panic("")
	}
	state.paused = !state.paused
	if w != nil {
		io.WriteString(*w, fmt.Sprintf("%d", vidTime(state)))
	}
}

/* Rewind the video by 10 seconds */
func rewind(state *cartoon) {
	var err error
	if state.player == nil {
		return
	}
	var secsRewound int64 = 5
	curTime := vidTime(state)
	rTime := curTime - secsRewound
	if rTime < 0 {
		err = state.player.Command([]string{"seek", "0", "absolute"})
	} else {
		err = state.player.Command([]string{"seek", "-5", "relative"})
	}
	if err != nil {
		fmt.Println("unable to seek ", err)
		panic("")
	}
}

/* Returns the time of the video in seconds */
func vidTime(state *cartoon) int64 {
	var vidTime int64
	for {
		t, _ := state.player.GetProperty("playback-time", mpv.FORMAT_INT64)
		if t != nil {
			vidTime = t.(int64)
			break
		}
	}
	return vidTime
}

func vidLen(state *cartoon) int64 {
	var vidLen int64
	// Wait for the video to initialize
	for {
		len, _ := state.player.GetProperty("duration", mpv.FORMAT_INT64)
		if len != nil {
			vidLen = len.(int64)
			break
		}
	}
	return vidLen
}

/*
func setVolume(state *cartoon, toVol int) {
	if state.player == nil {
		return
	}
	//state.player.SetVolume(toVol)
}
*/
func setTime(state *cartoon, seek int64, w *http.ResponseWriter) {
	if state.player == nil {
		startPlay(state, state.video)
	}
	err := state.player.Command([]string{"seek", strconv.FormatInt(seek, 10), "absolute"})
	if err != nil {
		fmt.Println("Unable to set the time ", err)
		panic("")
	}
	curTime := vidTime(state)
	if w != nil {
		io.WriteString(*w, fmt.Sprintf("%d", curTime))
	}
}

func screenshot(state *cartoon) {
	if state.player == nil {
		return
	}
	err := state.player.Command([]string{"screenshot"})
	if err != nil {
		fmt.Println("Unable to take screenshot ", err)
		panic("")
	}
}

func toggleSubs(state *cartoon) {
	var err error
	if state.player == nil {
		return
	}
	if state.subbed {
		err = state.player.SetOptionString("sub-visibility", "no")
	} else {
		err = state.player.SetOptionString("sub-visibility", "yes")
	}
	if err != nil {
		fmt.Println("Unable to toggle subs ", err)
		panic("")
	}
	state.subbed = !state.subbed
}

func main() {
	port := flag.String("p", "8100", "port to serve on")
	directory := flag.String("d", ".", "the directory of static file to host")
	flag.Parse()

	state := cartoon{
		player: nil,
		video:  "",
		vidLen: 0,
		paused: true,
		subbed: true,
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "list", 301)
	})
	http.Handle("/list/", http.StripPrefix("/list/", http.HandlerFunc(list)))
	http.Handle("/play/", http.StripPrefix("/play/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			/* Sometimes the unload event does not trigger on mobile.
			   Ensure that the other player has stopped. */
			if state.player != nil {
				// Seek to the end of the video and let the goroutine clean up the previous player
				setTime(&state, state.vidLen, nil)
				for {
					if state.player == nil {
						break
					}
				}
			}
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
			if state.paused {
				pausePlay(&state, nil)
			}
			// Seek to the end of the video and let the goroutine clean up the previous player
			setTime(&state, state.vidLen, nil)
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
			/*toVol, err := strconv.ParseInt(req.URL.String(), 10, 64)
			if err != nil {
				fmt.Println("NaN")
				return
			}
			setVolume(&state, int(toVol))
			*/
		})))
	http.Handle("/subs/", http.StripPrefix("/subs/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			toggleSubs(&state)
		})))
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})

	log.Printf("Serving %s on HTTP port: %s\n", *directory, *port)
	log.Fatal(http.ListenAndServe("192.168.2.8:"+*port, nil))
}
