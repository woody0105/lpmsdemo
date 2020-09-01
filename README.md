# LPMS WebSocket Demo for Speech Recognition
This proof of concept is a [lpms](https://github.com/livepeer/lpms) fork project to enable speech recognition in LPMS
It uses [deepspeech](https://github.com/mozilla/DeepSpeech) for speech to text transcription.

A Nvidia GPU (pascal or higher) is needed for real-time speech-to-text transcription.

To try this project as a standalone service, follow the instructions below.

### Requirements

Project requires libavcodec (ffmpeg) and friends. See `install_ffmpeg.sh` . Running this script will install everything in `~/compiled`. In order to build the project, the dependent libraries will need to be discoverable by pkg-config and golang. If you installed everything with `install_ffmpeg.sh` , then run `export PKG_CONFIG_PATH=~/compiled/lib/pkgconfig:$PKG_CONFIG_PATH` so the deps are picked up.
  
  remark: For rapid quality assurance we offer use of burned in subtitle. To use this ffmpeg should be built with --enable-libass

Current PoC uses deepspeechv0.8.2 prebuilt binaries and pretrained models. For GPU support, it requires cuda version 10.1. 

To build the project, you need to install golang.

https://golang.org/doc/install

### Build 

check PKG_CONFIG_PATH,CGO_CFLAGS,CGO_LDFLAGS environment values.

export PKG_CONFIG_PATH="${PKG_CONFIG_PATH:-}:$HOME/compiled/lib/pkgconfig"

export CGO_CFLAGS="-I$HOME/compiled/include"

export CGO_LDFLAGS="-L$HOME/compiled/lib"

```

git clone https://github.com/oscar-davids/lpmsdemo.git

cd lpmsdemo 

git checkout deepspeech_ws

go build example/deepspeech_websocket.go

```

