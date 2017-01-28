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
	"os/exec"
)

const videoPath string = "/home/kestein/Videos"

func play(w http.ResponseWriter, req *http.Request) {
	if len(req.URL.Path) == 1 {
		dir, err := os.Open(videoPath)
		if err != nil {
			log.Fatal("WRONG %s", err)
		}
		files, err := dir.Readdir(0)
		if err != nil {
			log.Fatal("WRONG %s", err)
		}
		for i := 0; i < len(files); i++ {
			vid := fmt.Sprintf("%s", files[i].Name())
			url := fmt.Sprintf("<a href=/%s>%s</a>\n", url.QueryEscape(vid), vid)
			io.WriteString(w, url)
		}
	} else {
		// Take everything after the '/' and play it in VLC
		rawVideo := string(req.URL.Path[1:])
		video, err := url.QueryUnescape(rawVideo)
		if err != nil {
			fmt.Println("WRONG")
		}
		cartoon := exec.Command("vlc", "-f", fmt.Sprintf("%s/%s", videoPath, video))
		err = cartoon.Start()
		if err != nil {
			log.Fatal("WRONG")
		}
	}
}

func main() {
	port := flag.String("p", "8100", "port to serve on")
	directory := flag.String("d", ".", "the directory of static file to host")
	flag.Parse()

	http.HandleFunc("/", play)

	log.Printf("Serving %s on HTTP port: %s\n", *directory, *port)
	log.Fatal(http.ListenAndServe("localhost:"+*port, nil))
}
