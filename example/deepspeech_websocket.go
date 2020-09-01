package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/oscar-davids/lpmsdemo/ffmpeg"
)

var addr = flag.String("addr", "localhost:8080", "http service address")
var seg_duration = 5
var upgrader = websocket.Upgrader{} // use default options

type ClientJob struct {
	recvdata []byte
	conn     *websocket.Conn
}

func speech2text(data []byte) string {
	result, _ := ffmpeg.DSSpeechToText1(data)
	return result
}

func processSeg(clientJobs chan ClientJob) {
	for {
		// Wait for the next job to come off the queue.
		clientJob := <-clientJobs
		clientJob.conn.WriteMessage(websocket.TextMessage, []byte("received segment process request"))
		// Send back the response.
		textres := speech2text(clientJob.recvdata)
		clientJob.conn.WriteMessage(websocket.TextMessage, []byte(textres))
	}
}

func handleaudiostream(w http.ResponseWriter, r *http.Request) {
	codec := r.Header.Get("X-WS-Audio-Codec")
	channel := r.Header.Get("X-WS-Audio-Channels")
	sample_rate := r.Header.Get("X-WS-Rate")
	bit_rate := r.Header.Get("X-WS-BitRate")

	if codec == "" || channel == "" || sample_rate == "" || bit_rate == "" {
		log.Print("audio meta data not present in header, handshake failed.")
		return
	}

	bit_rate_int, _ := strconv.Atoi(bit_rate)
	channel_int, _ := strconv.Atoi(channel)
	bytes_seg := bit_rate_int * channel_int * seg_duration / 8
	respheader := make(http.Header)
	respheader.Add("Sec-WebSocket-Protocol", "speechtotext.livepeer.com")
	c, err := upgrader.Upgrade(w, r, respheader)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	ffmpeg.CodecInit()
	defer ffmpeg.CodecDeinit()
	// sendchan := make(chan []byte)
	// recvchan := make(chan []byte)
	clientJobs := make(chan ClientJob)
	go processSeg(clientJobs)
	var recdata []byte
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		recdata = append(recdata, message...)
		if len(recdata) == bytes_seg {
			log.Println("received data bytes")
			c.WriteMessage(websocket.TextMessage, []byte("processing"))
			clientJobs <- ClientJob{recdata, c}
			recdata = nil
		}
	}

}

func main() {
	flag.Parse()
	log.SetFlags(0)
	ffmpeg.DSInit()
	// ffmpeg.AudioDecode()
	http.HandleFunc("/speech2text", handleaudiostream)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
