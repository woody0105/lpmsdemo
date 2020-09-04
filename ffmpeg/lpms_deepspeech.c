#include "lpms_deepspeech.h"

#include <libavcodec/avcodec.h>

#include <libavformat/avformat.h>
#include <libavfilter/avfilter.h>
#include <libavfilter/buffersink.h>
#include <libavfilter/buffersrc.h>
#include <libavutil/opt.h>
#include <libavutil/pixdesc.h>
#include <libswresample/swresample.h>

#include <pthread.h>

#include "libswscale/swscale.h"
#include "libavutil/imgutils.h"
#include "tensorflow/c/c_api.h"
#include "deepspeech/deepspeech.h"

SwrContext* resample_ctx = NULL;
ModelState* dsctx = NULL;      
Audioinfo cur_audio_input;
Audioinfo prev_audio_input;

int deepspeech_init(){
	const char *model = "deepspeech_models.pbmm";
	const char *scorer = "deepspeech_models.scorer";
  int status = DS_CreateModel(model, &dsctx);
  if (status != 0){
    av_log(0, AV_LOG_ERROR, "speech model create failed\n");    
    return -1;
  }

  status = DS_EnableExternalScorer(dsctx, scorer);
	if (status != 0) {
		fprintf(stderr, "Could not enable external scorer.\n");
		return 1;
	}

  av_log(0, AV_LOG_INFO, "Speech model created successfully.\n");
  return 0;
}

char* ds_stt(const short* aBuffer, unsigned int aBufferSize){
  if (dsctx == NULL){
    av_log(0, AV_LOG_INFO, "deepspeech model not loaded\n");
    return NULL;
  }
  char *textres = NULL;
  textres = (char *) malloc(1024);
  textres = DS_SpeechToText(dsctx, aBuffer, aBufferSize);
  av_log(0, AV_LOG_ERROR, "asr result: %s %u\n", textres, aBufferSize);
  return textres;
}

SwrContext* get_swrcontext(Audioinfo audio_input){
  SwrContext* resample_ctx = NULL;
  int output_channels = 1;
  int output_rate = 16000;
  enum AVSampleFormat output_sample_fmt = AV_SAMPLE_FMT_S16;
  resample_ctx = swr_alloc_set_opts(resample_ctx, av_get_default_channel_layout(output_channels),output_sample_fmt,output_rate,
                            av_get_default_channel_layout(audio_input.input_channels), audio_input.input_sample_fmt, audio_input.input_rate,0,NULL);
  if(!resample_ctx){
      printf("av_audio_resample_init fail!!!\n");
      return NULL;
  }
  swr_init(resample_ctx);
  av_log(0, AV_LOG_INFO, "resample context initialized\n");
  return resample_ctx;  
}

int compare_audioinfo(Audioinfo a, Audioinfo b){
  if( a.input_channels == b.input_channels && a.input_rate == b.input_rate && a.input_nb_samples == b.input_nb_samples && a.input_sample_fmt == b.input_sample_fmt)
    return 0;
  return -1;
}

#define AUDIO_INBUF_SIZE 512000 // 20480
#define AUDIO_REFILL_THRESH 4096

static int get_format_from_sample_fmt(const char **fmt,
                                      enum AVSampleFormat sample_fmt)
{
    int i;
    struct sample_fmt_entry {
        enum AVSampleFormat sample_fmt; const char *fmt_be, *fmt_le;
    } sample_fmt_entries[] = {
        { AV_SAMPLE_FMT_U8,  "u8",    "u8"    },
        { AV_SAMPLE_FMT_S16, "s16be", "s16le" },
        { AV_SAMPLE_FMT_S32, "s32be", "s32le" },
        { AV_SAMPLE_FMT_FLT, "f32be", "f32le" },
        { AV_SAMPLE_FMT_DBL, "f64be", "f64le" },
    };
    *fmt = NULL;

    for (i = 0; i < FF_ARRAY_ELEMS(sample_fmt_entries); i++) {
        struct sample_fmt_entry *entry = &sample_fmt_entries[i];
        if (sample_fmt == entry->sample_fmt) {
            *fmt = AV_NE(entry->fmt_be, entry->fmt_le);
            return 0;
        }
    }

    fprintf(stderr,
            "sample format %s is not supported as output format\n",
            av_get_sample_fmt_name(sample_fmt));
    return -1;
}

static void decode1(AVCodecContext *dec_ctx, AVPacket *pkt, AVFrame *frame,
                   ds_audio_buffer *audio_buffer)
{
    int i, ch;
    int ret, data_size;

    /* send the packet with the compressed data to the decoder */
    ret = avcodec_send_packet(dec_ctx, pkt);
    if (ret < 0) {
        fprintf(stderr, "Error submitting the packet to the decoder\n");
        exit(1);
    }

    /* read all the output frames (in general there may be any number of them */
    while (ret >= 0) {
        ret = avcodec_receive_frame(dec_ctx, frame);
        if (ret == AVERROR(EAGAIN) || ret == AVERROR_EOF)
            return;
        else if (ret < 0) {
            fprintf(stderr, "Error during decoding\n");
            exit(1);
        }
        data_size = av_get_bytes_per_sample(dec_ctx->sample_fmt);
        if (data_size < 0) {
            /* This should not occur, checking just for paranoia */
            fprintf(stderr, "Failed to calculate data size\n");
            exit(1);
        }
        
        cur_audio_input.input_channels = frame->channels;
        cur_audio_input.input_rate = frame->sample_rate;
        cur_audio_input.input_nb_samples = frame->nb_samples;
        cur_audio_input.input_sample_fmt = dec_ctx->sample_fmt;
        
        int output_channels = 1;
        int output_rate = 16000;
        enum AVSampleFormat output_sample_fmt = AV_SAMPLE_FMT_S16;
        
        if ( compare_audioinfo(cur_audio_input, prev_audio_input) != 0){
            if (resample_ctx != NULL){
              swr_free(&resample_ctx);
            }
            resample_ctx = get_swrcontext(cur_audio_input);
        }

        uint8_t* out_buffer = (uint8_t*)av_malloc(MAX_AUDIO_FRAME_SIZE);
        memset(out_buffer,0x00,sizeof(out_buffer));

        int out_nb_samples = av_rescale_rnd(swr_get_delay(resample_ctx, cur_audio_input.input_rate) + cur_audio_input.input_nb_samples, output_rate, cur_audio_input.input_rate, AV_ROUND_UP);

        int out_samples = swr_convert(resample_ctx,&out_buffer,out_nb_samples,(const uint8_t **)frame->data,frame->nb_samples);
        int out_buffer_size;
        if(out_samples > 0){
            out_buffer_size = av_samples_get_buffer_size(NULL,output_channels ,out_samples, output_sample_fmt, 1);
            memcpy(audio_buffer->buffer + audio_buffer->buffer_size, out_buffer, out_buffer_size);
            audio_buffer->buffer_size += out_buffer_size;
        }

        memcpy(&prev_audio_input, &cur_audio_input, sizeof(Audioinfo));
        av_free(out_buffer);
    }
}
const AVCodec *codec;
AVCodecContext *c= NULL;
AVCodecParserContext *parser = NULL;

void audio_codec_init()
{
    /* find the audio decoder */
    codec = avcodec_find_decoder(AV_CODEC_ID_AAC);
    if (!codec) {
        fprintf(stderr, "Codec not found\n");
        exit(1);
    }

    parser = av_parser_init(codec->id);
    if (!parser) {
        fprintf(stderr, "Parser not found\n");
        exit(1);
    }

    c = avcodec_alloc_context3(codec);
    if (!c) {
        fprintf(stderr, "Could not allocate audio codec context\n");
        exit(1);
    }

    /* open it */
    if (avcodec_open2(c, codec, NULL) < 0) {
        fprintf(stderr, "Could not open codec\n");
        exit(1);
    }
    printf("audio codec initialized.\n");
}

void audio_codec_deinit()
{
    avcodec_free_context(&c);
    av_parser_close(parser);
}
ds_audio_buffer residual_data = {0,};
ds_audio_buffer audio_buffer = {0,};
uint8_t inbuf[AUDIO_INBUF_SIZE + AV_INPUT_BUFFER_PADDING_SIZE+8192];
int totalbuffersize = 0;

ds_audio_buffer* decodeandresample(const char* aBuffer, unsigned int aBufferSize)
{
    int len, ret;
    
    uint8_t *data;
    size_t   data_size;
    AVPacket *pkt;
    AVFrame *decoded_frame = NULL;
    enum AVSampleFormat sfmt;
    int n_channels = 0;
    const char *fmt;

    pkt = av_packet_alloc();

    totalbuffersize = 0;
    audio_buffer.buffer = (char *)av_malloc(MAX_AUDIO_BUFFER_SIZE);
    printf("abuffsize %d\n", aBufferSize);

    if (residual_data.buffer != NULL && residual_data.buffer_size > 0){        
        memcpy(inbuf,residual_data.buffer,residual_data.buffer_size);
        totalbuffersize = residual_data.buffer_size;

        //if(totalbuffersize + aBufferSize < originalsize)
        memcpy(inbuf + totalbuffersize,aBuffer,aBufferSize);
        totalbuffersize += aBufferSize;

        av_free(residual_data.buffer);
    } else { //no residual
        printf("no residual\n");
        memcpy(inbuf ,aBuffer,aBufferSize);
        totalbuffersize = aBufferSize;
    }

    /* decode until eof */
    data      = inbuf;
    data_size = totalbuffersize;

    while (data_size > 0) {
        if (!decoded_frame) {
            if (!(decoded_frame = av_frame_alloc())) {
                fprintf(stderr, "Could not allocate audio frame\n");
                exit(1);
            }
        }
        ret = av_parser_parse2(parser, c, &pkt->data, &pkt->size,
                               data, data_size,
                               AV_NOPTS_VALUE, AV_NOPTS_VALUE, 0);

        if (ret < 0) {
            fprintf(stderr, "Error while parsing\n");   
            exit(1);
        }
        data      += ret;
        data_size -= ret;

        if (pkt->size)
            decode1(c, pkt, decoded_frame, &audio_buffer);

        if (data_size < AUDIO_REFILL_THRESH) {
            if(data_size > 0) {
                residual_data.buffer = (char *)av_malloc(data_size);
                printf("residual size %ld\n", data_size);
                memcpy(residual_data.buffer, data, data_size);
                residual_data.buffer_size = data_size;  
                break;
            } else if(data_size == 0){
                residual_data.buffer_size = 0;
                if(residual_data.buffer)
                    av_free(residual_data.buffer);  
            }
            
        }
    }
    
    /* flush the decoder */
    // pkt->data = NULL;
    // pkt->size = 0;
    // decode1(c, pkt, decoded_frame, &audio_buffer);
    /* print output pcm infomations, because there have no metadata of pcm */
    sfmt = c->sample_fmt;

    if (av_sample_fmt_is_planar(sfmt)) {
        const char *packed = av_get_sample_fmt_name(sfmt);
        printf("Warning: the sample format the decoder produced is planar "
               "(%s).\n",
               packed ? packed : "?");
        sfmt = av_get_packed_sample_fmt(sfmt);
    }

    n_channels = c->channels;
    if ((ret = get_format_from_sample_fmt(&fmt, sfmt)) < 0)
        goto end;
end:
    //avcodec_free_context(&c);
    //av_parser_close(parser);
    av_frame_free(&decoded_frame);
    av_packet_unref(pkt);

    return &audio_buffer;
}

char* ds_stt1(const char* aBuffer, unsigned int aBufferSize){
  if (dsctx == NULL){
    av_log(0, AV_LOG_INFO, "deepspeech model not loaded\n");
    return NULL;
  }

  ds_audio_buffer* audio_buffer = NULL;
  audio_buffer = decodeandresample(aBuffer, aBufferSize);  
  char *textres;
  textres = (char *) malloc(2048);
  textres = DS_SpeechToText(dsctx, audio_buffer->buffer, audio_buffer->buffer_size);
  av_free(audio_buffer->buffer);
  audio_buffer->buffer_size = 0;
  av_log(0, AV_LOG_ERROR, "asr result: %s \n", textres);
  return textres;
}
