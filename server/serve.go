package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
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

type videoLine struct {
	Action, Url, Filename string
}

type videoLineList []videoLine

func (a videoLineList) Len() int           { return len(a) }
func (a videoLineList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a videoLineList) Less(i, j int) bool { return a[i].Url < a[j].Url }

// TODO: Sanitize URLs so only subdirectories of videoPath can be queried
func list(w http.ResponseWriter, req *http.Request) {
	localURL := req.RequestURI[6:] // Take out the '/list'
	// ls the directory
	endDir, err := url.QueryUnescape(localURL)
	if err != nil {
		log.Fatal("Unable to parse video path %s", err)
	}
	dir, err := os.Open(fmt.Sprintf("%s/%s", videoPath, endDir))
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

func play(w http.ResponseWriter, req *http.Request) {
	// Take everything after the '/' and play it in VLC
	/*rawVideo := string(req.URL.Path[1:])
	video, err := url.QueryUnescape(rawVideo)
	if err != nil {
		fmt.Println("WRONG")
	}
	cartoon := exec.Command("vlc", "-f", fmt.Sprintf("%s/%s", videoPath, video))
	err = cartoon.Start()
	if err != nil {
		log.Fatal("WRONG")
	}*/
	fmt.Println("asdf")
}

func main() {
	port := flag.String("p", "8100", "port to serve on")
	directory := flag.String("d", ".", "the directory of static file to host")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "list", 301)
	})
	http.Handle("/list/", http.StripPrefix("/list/", http.HandlerFunc(list)))
	http.Handle("/play/", http.StripPrefix("/play/", http.HandlerFunc(play)))
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})

	log.Printf("Serving %s on HTTP port: %s\n", *directory, *port)
	log.Fatal(http.ListenAndServe("localhost:"+*port, nil))
}
