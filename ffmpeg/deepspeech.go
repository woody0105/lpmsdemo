package ffmpeg

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/gorilla/websocket"
)

// #cgo pkg-config: libavformat libavfilter libavcodec libavutil libswscale gnutls
// #cgo LDFLAGS: -ldeepspeech
// #include <stdlib.h>
// #include "lpms_deepspeech.h"
import "C"

type TimedPacket struct {
	Packetdata APacket
	Timestamp  uint64
}

type APacket struct {
	Data   []byte
	Length int
}

type Transcriber struct {
	Id           string
	Conn         *websocket.Conn
	handle       *C.struct_transcribe_thread
	codec_params *C.codec_params
	modelState   *C.ModelState
	streamState  *C.StreamingState
	stopped      bool
	mu           *sync.Mutex
}

func DSInit() int {
	C.deepspeech_init()
	fmt.Println("deepspeech initialized")
	return 0
}

func DSSpeechToText(data []byte) (string, error) {
	datalength := len(data) / 2

	buffer := (*C.short)(unsafe.Pointer(C.CString(string(data))))
	defer C.free(unsafe.Pointer(buffer))
	str := C.ds_stt(buffer, C.uint(datalength))

	if str == nil {
		return "", errors.New("conversion failed")
	}

	defer C.free(unsafe.Pointer(str))

	return C.GoString(str), nil
}

func DSSpeechToText1(data []byte) (string, error) {
	datalength := len(data)

	buffer := (*C.char)(unsafe.Pointer(C.CString(string(data))))
	defer C.free(unsafe.Pointer(buffer))
	str := C.ds_stt1(buffer, C.uint(datalength))

	if str == nil {
		return "", errors.New("conversion failed")
	}

	defer C.free(unsafe.Pointer(str))

	return C.GoString(str), nil
}

func FeedPacket(pkt TimedPacket) string {
	pktdata := pkt.Packetdata
	buffer := (*C.char)(unsafe.Pointer(C.CString(string(pktdata.Data))))
	defer C.free(unsafe.Pointer(buffer))
	str := C.ds_feedpkt(buffer, C.int(pktdata.Length))
	return C.GoString(str)
}

func CodecInit() {
	C.audio_codec_init()
}

func CodecDeinit() {
	C.audio_codec_deinit()
}

func NewTranscriber(Id string) *Transcriber {
	t := &Transcriber{
		Id:           Id,
		codec_params: C.lpms_codec_new(),
		mu:           &sync.Mutex{},
	}
	t.modelState = C.t_deepspeech_init()
	t.streamState = C.t_create_stream(t.modelState)
	return t
}

func (t *Transcriber) StopTranscriber() {
	C.lpms_codec_stop(t.codec_params)
	// C.t_free_model(t.modelState, t.streamState)
	t = nil
}

func (t *Transcriber) TranscriberCodecInit() {
	codec_params := t.codec_params
	C.t_audio_codec_init(codec_params)
}

func (t *Transcriber) TranscriberCodecDeinit() {
	codec_params := t.codec_params
	C.t_audio_codec_deinit(codec_params)
}

func (t *Transcriber) FeedPacket(pkt TimedPacket) string {
	pktdata := pkt.Packetdata
	codec_params := t.codec_params
	model_ctx := t.modelState
	stream_ctx := t.streamState
	buffer := (*C.char)(unsafe.Pointer(C.CString(string(pktdata.Data))))
	defer C.free(unsafe.Pointer(buffer))
	str := C.t_ds_feedpkt(codec_params, model_ctx, stream_ctx, buffer, C.int(pktdata.Length))
	return C.GoString(str)
}
