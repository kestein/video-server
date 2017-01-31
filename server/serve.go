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
	//"os/exec"
)

const videoPath string = "/home/kestein/Videos"

func list(w http.ResponseWriter, req *http.Request) {
	dir, err := os.Open(videoPath)
	if err != nil {
		log.Fatal("Unable to open video path %s", err)
	}
	files, err := dir.Readdir(0)
	if err != nil {
		log.Fatal("Unable to read dir %s", err)
	}
	io.WriteString(w, "<html><body><ul>\n")
	for i := 0; i < len(files); i++ {
		vid := fmt.Sprintf("%s", files[i].Name())
		url := fmt.Sprintf("<li><a href=/play/%s>%s</a></li>\n", url.QueryEscape(vid), vid)
		io.WriteString(w, url)
	}
	io.WriteString(w, "</ul></body></html>\n")
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
	http.Handle("/list/", http.StripPrefix("/list/", http.HandlerFunc(list)))
	http.Handle("/play/", http.StripPrefix("/play/", http.HandlerFunc(play)))

	log.Printf("Serving %s on HTTP port: %s\n", *directory, *port)
	log.Fatal(http.ListenAndServe("localhost:"+*port, nil))
}
