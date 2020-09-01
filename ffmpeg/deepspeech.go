package ffmpeg

import (
	"errors"
	"fmt"
	"unsafe"
)

// #cgo pkg-config: libavformat libavfilter libavcodec libavutil libswscale gnutls
// #cgo LDFLAGS: -ldeepspeech
// #include <stdlib.h>
// #include "lpms_deepspeech.h"
import "C"

func DSInit() int {
	C.deepspeech_init()
	fmt.Println("deepspeech initialized")
	return 0
}

func DSSpeechToText(data []byte) (string, error) {
	// var str *C.char
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
	// var str *C.char
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

// func AudioDecode() int {
// 	C.test()
// 	return 0
// }
func CodecInit() {
	C.audio_codec_init()
}

func CodecDeinit() {
	C.audio_codec_deinit()
}
