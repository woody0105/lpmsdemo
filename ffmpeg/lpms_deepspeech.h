#ifndef _LPMS_DEEPSPEECH_H_
#define _LPMS_DEEPSPEECH_H_
#include <libavutil/hwcontext.h>
#include <libavutil/rational.h>
#include <libswresample/swresample.h>

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

int deepspeech_init();
char* ds_stt(const short* aBuffer, unsigned int aBufferSize);
char* ds_stt1(const char* aBuffer, unsigned int aBufferSize);
SwrContext* get_swrcontext(Audioinfo audio_input);
int compare_audioinfo(Audioinfo a, Audioinfo b);

int test();
void audio_codec_init();
void audio_codec_deinit();
ds_audio_buffer* decodeandresample(const char* aBuffer, unsigned int aBufferSize);
#endif

