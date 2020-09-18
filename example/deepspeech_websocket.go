package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oscar-davids/lpmsdemo/ffmpeg"
)

var addr = flag.String("addr", "localhost:8080", "http service address")
var seg_duration = 5
var upgrader = websocket.Upgrader{} // use default options

type ClientJob struct {
	// recvdata []byte
	recvpackets []ffmpeg.TimedPacket
	conn        *websocket.Conn
}

func speech2text(data []byte) string {
	result, _ := ffmpeg.DSSpeechToText1(data)
	return result
}

func processSeg(clientJobs chan ClientJob) {
	for {
		// Wait for the next job to come off the queue.
		clientJob := <-clientJobs
		timestamp := clientJob.recvpackets[0].Timestamp
		recvpackets := clientJob.recvpackets
		var apacketsdata []byte
		for _, recvpacket := range recvpackets {
			apacketsdata = append(apacketsdata, recvpacket.Packetdata.Data...)
		}
		start := time.Now()
		textres := speech2text(apacketsdata)
		duration := time.Since(start)
		fmt.Println("Elapsed time:", duration)
		res := map[string]interface{}{"timestamp": timestamp, "text": textres}
		jsonres, _ := json.Marshal(res)
		// clientJob.conn.WriteMessage(websocket.TextMessage, []byte(textres))
		fmt.Printf("json %s\n", string(jsonres))
		clientJob.conn.WriteMessage(websocket.TextMessage, []byte(string(jsonres)))
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

	var last string
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		timestamp := binary.BigEndian.Uint64(message[:8])
		packetdata := message[8:]
		timedpacket := ffmpeg.TimedPacket{Timestamp: timestamp, Packetdata: ffmpeg.APacket{packetdata, len(packetdata)}}
		str := ffmpeg.FeedPacket(timedpacket)

		if strings.Compare(last, str) != 0 {
			last = str
			res := map[string]interface{}{"timestamp": timestamp, "text": last}
			jsonres, _ := json.Marshal(res)
			fmt.Println(string(jsonres))
			c.WriteMessage(websocket.TextMessage, []byte(string(jsonres)))
		}
	}

}

func main() {
	flag.Parse()
	log.SetFlags(0)
	ffmpeg.DSInit()
	http.HandleFunc("/speech2text", handleaudiostream)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
