package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/woody0105/segmentation/ffmpeg"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

type Client struct {
	ID   string
	Conn *websocket.Conn
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients map[*websocket.Conn]bool

func handleconnections1(w http.ResponseWriter, r *http.Request) {

	codec := r.Header.Get("X-WS-Codec")
	size := r.Header.Get("X-WS-Video-Size")
	sizetmp := strings.Split(size, "x")
	width, _ := strconv.Atoi(sizetmp[0])
	height, _ := strconv.Atoi(sizetmp[1])

	respheader := make(http.Header)
	initData := r.Header.Get("X-Ws-Init")
	spsData, _ := base64.StdEncoding.DecodeString(initData)
	respheader.Add("Sec-WebSocket-Protocol", "videoprocessing.livepeer.com")
	c, err := upgrader.Upgrade(w, r, respheader)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	fmt.Println("video codec id:", codec)

	ffmpeg.SetDecoderCtxParams(width, height)
	handlemsg1(w, r, c, codec, spsData)

}

func handlemsg1(w http.ResponseWriter, r *http.Request, conn *websocket.Conn, codec string, initData []byte) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			log.Printf("read:%v", err)
			// conn.Close()
			break
		}
		timestamp := binary.BigEndian.Uint64(message[:8])
		packetdata := message[8:]
		nalHeader := binary.BigEndian.Uint64(packetdata[:8])

		nalUnitType := nalHeader % 32
		if nalUnitType == 5 {
			packetdata = append(initData, packetdata...)
		}
		timedpacket := ffmpeg.TimedPacket{Timestamp: timestamp, Packetdata: ffmpeg.APacket{Data: packetdata, Length: len(packetdata)}}
		inferRes := ffmpeg.FeedPacket(timedpacket)
		if inferRes != "" {
			res := map[string]interface{}{"timestamp": int(timestamp), "metadata": inferRes, "type": "metadata"}
			jsonres, _ := json.Marshal(res)
			log.Println("writing data, timestamp:", timestamp)
			conn.WriteMessage(websocket.TextMessage, []byte(string(jsonres)))
		}
	}
}

func startServer1() {
	log.Println("started server", *addr)
	http.HandleFunc("/segmentation", handleconnections1)
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	ffmpeg.DecoderInit()
	startServer1()
	log.Fatal(http.ListenAndServe(*addr, nil))
}
