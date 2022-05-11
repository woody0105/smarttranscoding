# Livepeer Realtime Audio Transcription

Livepeer Realtime Audio Transcription is a real time speech to text PoC that produces transcription of audio broadcast.
It is based on Mozilla's [deepspeech](https://github.com/mozilla/DeepSpeech), which is the tensorflow implementation based on [Baidu's Deep Speech research paper](https://arxiv.org/abs/1412.5567) for speech to text transcription.

A Nvidia GPU (pascal or higher) is needed for real-time speech-to-text transcription.

To try this project as a standalone service, follow the instructions below.

### Architecture
 
 ![Alt text](drawings/workflow.png?raw=true "")

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

Clone the repository

```
git clone https://github.com/woody0105/LRAT.git

```

Copy deepspeech library and header files and build.

```
cd LRAT

cp -r deepspeech/deepspeech $HOME/compiled/include/
cp deepspeech/libdeepspeech.so $HOME/compiled/lib/  

go build example/deepspeech_websocket.go

```

### Test
Run the websocket server to listen on port 8081.
```
./deepspeech_websocket -addr=0.0.0.0:8081
```

Using mist server, stream audio packets to the websocket server.



