/*
Serve is a very simple static file server in go
Usage:
	-p="8100": port to serve on
	-d=".":    the directory of static files to host

Navigating to http://localhost:8100 will display the index.html or directory
listing file.
*/
package main

import (
	"flag"
	"fmt"
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
	subDirs := []string{}
	orphanFiles := []string{}
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
			url := fmt.Sprintf("<li><a href=/list/%s>%s</a></li>\n", vidURL, fileName)
			subDirs = append(subDirs, url)
		} else {
			url := fmt.Sprintf("<li><a href=/play/%s>%s</a></li>\n", vidURL, fileName)
			orphanFiles = append(orphanFiles, url)
		}
	}
	// Write HTML to the reponse
	sort.Strings(subDirs)
	// Make the '..' directory at the top
	prev := strings.Split(req.URL.String(), "/")
	back := ""
	if len(prev) > 1 {
		back = strings.Join(prev[:len(prev)-1], "/")
	}
	previousLink := []string{fmt.Sprintf("<li><a href=/list/%s>..</a></li>\n", back)}
	subDirs = append(previousLink, subDirs...)
	sort.Strings(orphanFiles)
	// Write the HTML
	htmlBody := fmt.Sprintf("<html><body><ul>\n%s%s</ul></body></html>\n", strings.Join(subDirs, "\n"), strings.Join(orphanFiles, "\n"))
	io.WriteString(w, htmlBody)
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

	http.Handle("/", http.StripPrefix("/", http.HandlerFunc(list)))
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})
	http.Handle("/list/", http.StripPrefix("/list/", http.HandlerFunc(list)))
	http.Handle("/play/", http.StripPrefix("/play/", http.HandlerFunc(play)))

	log.Printf("Serving %s on HTTP port: %s\n", *directory, *port)
	log.Fatal(http.ListenAndServe("localhost:"+*port, nil))
}
