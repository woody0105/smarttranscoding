package ffmpeg

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"unsafe"
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

var i int = 0

func DecoderInit() {
	C.video_codec_init()
	fmt.Println("decoder initialized")
}

func SetDecoderCtxParams(w int, h int) {
	C.set_decoder_ctx_params(C.int(w), C.int(h))
}

func FeedPacket(pkt TimedPacket) string {
	pktdata := pkt.Packetdata
	buffer := (*C.char)(unsafe.Pointer(C.CString(string(pktdata.Data))))
	defer C.free(unsafe.Pointer(buffer))
	C.ds_feedpkt(buffer, C.int(pktdata.Length), C.int(pkt.Timestamp))

	path, _ := os.Getwd()
	filename := filepath.Join(path, "frame"+strconv.Itoa(int(pkt.Timestamp))+".ppm")
	defer os.Remove(filename)
	retStr := ""
	var client http.Client
	url := "http://localhost:6337/detect_objects?image_path=" + filename
	if i%5 == 0 {
		resp, err := client.Get(url)
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
			// for debugging
			// dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(retStr))
			// jpgI, errJpg := jpeg.Decode(dec)
			// if errJpg == nil {
			// 	f, _ := os.OpenFile("res"+strconv.Itoa(int(pkt.Timestamp))+".jpg", os.O_WRONLY|os.O_CREATE, 0777)
			// 	jpeg.Encode(f, jpgI, &jpeg.Options{Quality: 75})
			// 	fmt.Println("Jpg created")
			// } else {
			// 	fmt.Println(errJpg.Error())
			// }

		}
	}
	i = (i + 1) % 5

	return retStr
}
