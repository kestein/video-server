package main

import (
	"flag"
	"fmt"
	vlc "github.com/jteeuwen/go-vlc"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const videoPath string = "/home/kestein/Videos"
const referer string = "Referer"
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
const playerPage string = `
<html>
	<script>
		window.onload = function () {
			var playback = document.getElementById("playback");
			playback.addEventListener("click", function() {
				var x = new XMLHttpRequest();
				x.onreadystatechange = function() {

				};
				x.open("GET", "/playback/", true);
				x.send();
			});

			var stop = document.getElementById("stop");
			stop.addEventListener("click", function() {
				var x = new XMLHttpRequest();
				x.onreadystatechange = function() {

				};
				x.open("GET", "/stop/", true);
				x.send();
			});
		};
	</script>
	<body>
		<button id="playback">Pause</button>
		<button id="stop">Stop</button>
	</body>
</html>
`

type videoLine struct {
	Action, Url, Filename string
}

type cartoon struct {
	inst    *vlc.Instance
	player  *vlc.Player
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
	rawVideo := string(req.URL.Path[0:])
	video, err := url.QueryUnescape(rawVideo)
	if err != nil {
		fmt.Println("WRONG")
	}
	// Make VLC instance
	if state.inst, err = vlc.New([]string{}); err != nil {
		fmt.Println(err)
	}
	// Open the video file
	if media, err = state.inst.OpenMediaFile(fmt.Sprintf("%s/%s", videoPath, video)); err != nil {
		fmt.Println(err)
	}
	// Create the media player
	if state.player, err = media.NewPlayer(); err != nil {
		fmt.Println(err)
	}
	// Do not need media anymore since player now owns it
	media.Release()
	media = nil

	// Make the page to control the video
	state.player.Play()
	state.playing = true
	io.WriteString(w, playerPage)
}

func stop(state *cartoon, w http.ResponseWriter, req *http.Request) {
	if state.player == nil {
		return
	}
	state.player.Stop()
	state.playing = false
	closePlayer(state)
}

func closePlayer(state *cartoon) {
	state.player.Release()
	state.inst.Release()
	state.player = nil
	state.inst = nil
}

func main() {
	port := flag.String("p", "8100", "port to serve on")
	directory := flag.String("d", ".", "the directory of static file to host")
	flag.Parse()

	state := cartoon{
		inst:    nil,
		player:  nil,
		playing: false,
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
			state.player.TogglePause(!state.playing)
			state.playing = !state.playing
		})))
	http.Handle("/stop/", http.StripPrefix("/stop/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			stop(&state, w, req)
		})))
	http.HandleFunc("/info/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(state.inst)
	})
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})

	log.Printf("Serving %s on HTTP port: %s\n", *directory, *port)
	log.Fatal(http.ListenAndServe("localhost:"+*port, nil))
}
