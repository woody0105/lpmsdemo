#ifndef _LPMS_DEEPSPEECH_H_
#define _LPMS_DEEPSPEECH_H_
#include <libavutil/hwcontext.h>
#include <libavutil/rational.h>
#include <libswresample/swresample.h>
#include "deepspeech/deepspeech.h"
#ifndef MAX_AUDIO_FRAME_SIZE
#define MAX_AUDIO_FRAME_SIZE 32000
#endif

#ifndef MAX_AUDIO_BUFFER_SIZE
#define MAX_AUDIO_BUFFER_SIZE 1024000
#endif


#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>

typedef struct Audioinfo{
    int input_channels;
    int input_rate;
    int input_nb_samples;
    enum AVSampleFormat input_sample_fmt;
} Audioinfo;

typedef struct {
	char*  buffer;
	size_t buffer_size;
} ds_audio_buffer;

#define MAX_STACK_SIZE 0x1000000
#define PROCESSOR_UNIT 640

typedef struct {
    char pbuffer[MAX_STACK_SIZE];
    int nPos;
} STACK;

typedef struct {
    int initialized;
} struct_transcribe_thread;

typedef struct {
    // SwrContext* resample_ctx;
    const AVCodec *codec;
    AVCodecContext *c;
} codec_params;

int deepspeech_init();
char* ds_stt(const short* aBuffer, unsigned int aBufferSize);
char* ds_stt1(const char* aBuffer, unsigned int aBufferSize);
SwrContext* get_swrcontext(Audioinfo audio_input);
int compare_audioinfo(Audioinfo a, Audioinfo b);

void audio_codec_init();
void audio_codec_deinit();
void create_dsstream();
char* finish_dsstream();
ds_audio_buffer* decodeandresample(const char* aBuffer, unsigned int aBufferSize);
char* ds_feedpkt(const char* pktdata, int pktsize);

codec_params* lpms_codec_new();
void lpms_codec_stop(codec_params* h);
ModelState* t_deepspeech_init();
StreamingState* t_create_stream();
void t_free_model(StreamingState *stream_ctx);
void t_audio_codec_init(codec_params *codec_params);
void t_audio_codec_deinit(codec_params *codec_params);
StreamingState* t_ds_feedpkt(codec_params *codec_params, StreamingState *stream_ctx, char* pktdata, int pktsize, char* textres);
#endif

