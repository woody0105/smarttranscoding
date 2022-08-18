package ffmpeg

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"unsafe"

	"github.com/gorilla/websocket"
)

// #cgo pkg-config: libavformat libavfilter libavcodec libavutil libswscale gnutls
// #include <stdlib.h>
// #include "lpms_ffmpeg.h"
import "C"

type TimedPacket struct {
	Packetdata APacket
	Timestamp  uint64
}

type APacket struct {
	Data   []byte
	Length int
}

type Instance struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Image    string `json:"image"`
	MetaData string `json:"metadata"`
	Action   string `json:"action"`
}

var i int = 0

func DecoderInit() {
	C.video_codec_init()
	fmt.Println("decoder initialized")
}

func SetDecoderCtxParams(w int, h int) {
	C.set_decoder_ctx_params(C.int(w), C.int(h))
}

func FeedPacket(pkt TimedPacket, nodes []string, conn *websocket.Conn) {
	timestamp := pkt.Timestamp
	pktdata := pkt.Packetdata
	buffer := (*C.char)(unsafe.Pointer(C.CString(string(pktdata.Data))))
	defer C.free(unsafe.Pointer(buffer))
	C.ds_feedpkt(buffer, C.int(pktdata.Length), C.int(pkt.Timestamp))

	path, _ := os.Getwd()
	filename := filepath.Join(path, "frame"+strconv.Itoa(int(pkt.Timestamp))+".jpg")
	defer os.Remove(filename)
	retStr := ""
	client := &http.Client{}

	url := ""
	for _, node := range nodes {
		url = fmt.Sprintf("http://%s:6337/face-recognition", node)
	}

	trackid := "2"
	if i%8 == 4 {
		f, _ := os.Open(filename)

		// Read entire JPG into byte slice.
		reader := bufio.NewReader(f)
		content, _ := ioutil.ReadAll(reader)

		// Encode as base64.
		encoded := base64.StdEncoding.EncodeToString(content)

		// Print encoded data to console.
		// ... The base64 image can be used as a data URI in a browser.
		// fmt.Println("ENCODED: " + encoded)
		metadata := fmt.Sprintf(`{"image": "data:image/jpg;base64, %s"}`, encoded)
		// fmt.Println(encoded, "\n\n\n\n")
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte(metadata)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			bodyString := string(bodyBytes)
			retStr = bodyString
		}

		trackid = "3"

		res := map[string]interface{}{"trackid": trackid, "timestamp": int(timestamp), "metadata": retStr, "type": "metadata"}
		jsonres, _ := json.Marshal(res)
		log.Println("writing data, timestamp:", timestamp, "\n", string(jsonres))
		conn.WriteMessage(websocket.TextMessage, []byte(string(jsonres)))

	} else if i%40 == 0 {
		url = "http://34.121.62.161:6337/image-captioning"
		f, _ := os.Open(filename)

		// Read entire JPG into byte slice.
		reader := bufio.NewReader(f)
		content, _ := ioutil.ReadAll(reader)

		// Encode as base64.
		encoded := base64.StdEncoding.EncodeToString(content)

		// Print encoded data to console.
		// ... The base64 image can be used as a data URI in a browser.
		// fmt.Println("ENCODED: " + encoded)
		metadata := fmt.Sprintf(`{"image": "data:image/jpg;base64, %s"}`, encoded)
		// fmt.Println(encoded, "\n\n\n\n")
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte(metadata)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			bodyString := string(bodyBytes)
			retStr = bodyString
		}

		trackid = "2"

		res := map[string]interface{}{"trackid": trackid, "timestamp": int(timestamp), "metadata": retStr, "type": "metadata"}
		jsonres, _ := json.Marshal(res)
		log.Println("writing data, timestamp:", timestamp, "\n", string(jsonres))
		conn.WriteMessage(websocket.TextMessage, []byte(string(jsonres)))

	}
	i = (i + 1) % 30
}

func RegisterSamples(registerData *bytes.Buffer) (*http.Response, error) {
	client := &http.Client{}

	// fmt.Println(registerData)
	url := "http://127.0.0.1:6337/update-samples"
	// resp, err := client.Post(url, "application/json", registerData)

	req, _ := http.NewRequest("POST", url, registerData)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))
	return resp, err
}
