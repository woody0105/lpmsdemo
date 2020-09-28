package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/oscar-davids/lpmsdemo/ffmpeg"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

type Client struct {
	ID   string
	Conn *websocket.Conn
}

type message struct {
	clientid string
	data     []byte
}

var msgchan = make(chan message)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*Client]bool) // connected clients
var transcribers = make(map[*ffmpeg.Transcriber]bool)

var RandomIDGenerator = func(length uint) string {
	x := make([]byte, length, length)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return hex.EncodeToString(x)
}

func RandName() string {
	return RandomIDGenerator(10)
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

func handleconnections(w http.ResponseWriter, r *http.Request) {
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

	clientId := RandName()

	t := ffmpeg.NewTranscriber(clientId)
	t.Conn = c
	t.TranscriberCodecInit()
	defer t.TranscriberCodecDeinit()
	defer t.StopTranscriber()

	// Register our new transcriber
	transcribers[t] = true

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			delete(transcribers, t)
			break
		}
		recvmsg := message{clientId, msg}
		// Send the newly received message to the channel
		msgchan <- recvmsg
	}
}

func handlemsg() {
	for {
		// Grab the next message from the channel
		msg := <-msgchan
		timestamp := binary.BigEndian.Uint64(msg.data[:8])
		packetdata := msg.data[8:]
		timedpacket := ffmpeg.TimedPacket{Timestamp: timestamp, Packetdata: ffmpeg.APacket{packetdata, len(packetdata)}}
		// str := ffmpeg.FeedPacket(timedpacket)
		// log.Printf("processed timestamp: %d", timestamp)
		// Send it out to the right client
		for t := range transcribers {
			if t.Id == msg.clientid {
				str := t.FeedPacket(timedpacket)
				res := map[string]interface{}{"timestamp": timestamp, "text": str}
				jsonres, _ := json.Marshal(res)
				fmt.Println(string(jsonres))
				t.Conn.WriteMessage(websocket.TextMessage, []byte(string(jsonres)))
			}
		}
	}
}

func startServer() {
	http.HandleFunc("/speech2text", handleconnections)
	go handlemsg()
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	// ffmpeg.DSInit()
	// http.HandleFunc("/speech2text", handleconnections)
	startServer()
	log.Fatal(http.ListenAndServe(*addr, nil))
}
