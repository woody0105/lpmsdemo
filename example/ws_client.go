package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/speech2text"}
	wsHeaders := http.Header{
		"X-WS-Audio-Codec":    {"aac"},
		"X-WS-Audio-Channels": {"2"},
		"X-WS-Rate":           {"44100"},
		"X-WS-BitRate":        {"98000"},
	}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), wsHeaders)
	if err != nil {
		log.Fatal("dial:", err)
	}
	// defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	ticker := time.NewTicker(time.Microsecond)
	defer ticker.Stop()

	dat, err := ioutil.ReadFile("test.aac")

	// error := c.WriteMessage(websocket.BinaryMessage, dat)
	// if error != nil {
	// 	log.Println("write:", error)
	// 	return
	// }
	var count int
	datalen := len(dat) - 1
	// datalen := 256000
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if count >= datalen {
				log.Printf("end of data")
				// time.Sleep(100 * time.Second)
				select {
				case <-done:
				case <-time.After(time.Minute):
				}
				return
			}
			senddata := make([]byte, 30)
			senddata = dat[count : count+30]
			err := c.WriteMessage(websocket.BinaryMessage, senddata)
			count += 30
			if err != nil {
				log.Println("write:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Millisecond):
			}
			return
		}
	}
}
