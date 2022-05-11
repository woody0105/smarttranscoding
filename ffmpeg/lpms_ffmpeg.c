#include "lpms_ffmpeg.h"

#include <libavcodec/avcodec.h>

#include <libavformat/avformat.h>
#include <libavfilter/avfilter.h>
#include <libavfilter/buffersink.h>
#include <libavfilter/buffersrc.h>
#include <libavutil/opt.h>
#include <libavutil/pixdesc.h>

#include <pthread.h>
#include <unistd.h>

#include "libswscale/swscale.h"
#include "libavutil/imgutils.h"

AVCodec *decoder = NULL;
AVCodecContext *decoder_ctx = NULL;
AVCodecParserContext *parser = NULL;

static AVBufferRef *hw_device_ctx = NULL;
static enum AVPixelFormat hw_pix_fmt;
enum AVHWDeviceType device_type;

static int hw_decoder_init(AVCodecContext *ctx, const enum AVHWDeviceType type)
{
    int err = 0;

    if ((err = av_hwdevice_ctx_create(&hw_device_ctx, type,
                                      NULL, NULL, 0)) < 0) {
        fprintf(stderr, "Failed to create specified HW device.\n");
        return err;
    }
    ctx->hw_device_ctx = av_buffer_ref(hw_device_ctx);

    return err;
}

static enum AVPixelFormat get_hw_format(AVCodecContext *ctx,
                                        const enum AVPixelFormat *pix_fmts)
{
    const enum AVPixelFormat *p;

    for (p = pix_fmts; *p != -1; p++) {
        if (*p == hw_pix_fmt)
            return *p;
    }

    fprintf(stderr, "Failed to get HW surface format.\n");
    return AV_PIX_FMT_NONE;
}

static void SaveFrame(AVFrame *pFrame, int width, int height, int iFrame)
{
    FILE *pFile;
    char szFilename[32];
    int  y;

    // Open file
    sprintf(szFilename, "frame%d.ppm", iFrame);
    pFile=fopen(szFilename, "wb");
    if(pFile==NULL)
        return;

    // Write header
    fprintf(pFile, "P6\n%d %d\n255\n", width, height);

    // Write pixel data
    for(y=0; y<height; y++)
        fwrite(pFrame->data[0]+y*pFrame->linesize[0], 1, width*3, pFile);

    // Close file
    fclose(pFile);
}

void video_codec_init()
{
    int i;
    device_type = AV_HWDEVICE_TYPE_CUDA;
    /* find the video decoder */
    decoder = avcodec_find_decoder_by_name("h264_cuvid");
    if (!decoder) {
        fprintf(stderr, "Codec not found\n");
        exit(1);
    }


    for (i = 0;; i++) {
        const AVCodecHWConfig *config = avcodec_get_hw_config(decoder, i);
        if (!config) {
            fprintf(stderr, "Decoder %s does not support device type %s.\n",
                    decoder->name, av_hwdevice_get_type_name(device_type));
            return;
        }
        if (config->methods & AV_CODEC_HW_CONFIG_METHOD_HW_DEVICE_CTX &&
            config->device_type == device_type) {
            hw_pix_fmt = config->pix_fmt;
            break;
        }
    }

    decoder_ctx = avcodec_alloc_context3(decoder);
    if (!decoder_ctx) {
        fprintf(stderr, "Could not allocate codec context\n");
        exit(1);
    }

    // First set the hw device then set the hw frame
    decoder_ctx->get_format  = get_hw_format;

    int err = 0;

    if ((err = av_hwdevice_ctx_create(&hw_device_ctx, device_type,
                                      NULL, NULL, 0)) < 0) {
        fprintf(stderr, "Failed to create specified HW device.\n");
        return;
    }
    decoder_ctx->hw_device_ctx = av_buffer_ref(hw_device_ctx);

    /* open it */
    if (avcodec_open2(decoder_ctx, decoder, NULL) < 0) {
        fprintf(stderr, "Could not open codec\n");
        exit(1);
    }
    decoder_ctx->width = 1280;
    decoder_ctx->height = 720;
    printf("video codec initialized.\n");
}


void set_decoder_ctx_params(int w, int h)
{
    decoder_ctx->width = w;
    decoder_ctx->height = h;
}

int i = 0;
int firstTime = 0;
struct SwsContext* swsContext = NULL;

#define RESCALE 1 

void decode_feed(AVCodecContext *dec_ctx, AVPacket *pkt, AVFrame *frame, int timestamp)
{
    // int i, ch;
    int ret, data_size;
    AVFrame *pFrameRGB;
    AVFrame *swFrame;
    pFrameRGB = av_frame_alloc();
    swFrame = av_frame_alloc();
    if (NULL == pFrameRGB || NULL == swFrame) {
        fprintf(stderr, "Alloc frame failed!\n");
        return;
    }

    uint8_t *buffer = NULL;
    int numBytes = 0;

    // calculate buffer size after decoding and allocate buffer
    #if !RESCALE
    numBytes = av_image_get_buffer_size(AV_PIX_FMT_RGB24, decoder_ctx->width, decoder_ctx->height, 1);
    buffer = (uint8_t *)av_malloc(numBytes * sizeof(uint8_t));
    av_image_fill_arrays(pFrameRGB->data, pFrameRGB->linesize, buffer, AV_PIX_FMT_RGB24, decoder_ctx->width, decoder_ctx->height, 1);
    #else
    numBytes = av_image_get_buffer_size(AV_PIX_FMT_RGB24, 400, 300, 1);
    buffer = (uint8_t *)av_malloc(numBytes * sizeof(uint8_t));
    av_image_fill_arrays(pFrameRGB->data, pFrameRGB->linesize, buffer, AV_PIX_FMT_RGB24, 400, 300, 1);
    #endif
    /* send the packet with the compressed data to the decoder */
    ret = avcodec_send_packet(dec_ctx, pkt);
    if (ret < 0) {
        fprintf(stderr, "Error submitting the packet to the decoder\n");
        goto fail;
    }
    if (firstTime == 0) {
        firstTime = 1;
        #if !RESCALE
        swsContext = sws_getContext(decoder_ctx->width, decoder_ctx->height, AV_PIX_FMT_NV12, decoder_ctx->width, decoder_ctx->height, AV_PIX_FMT_RGB24, SWS_BICUBIC, NULL, NULL, NULL);
        #else
        swsContext = sws_getContext(decoder_ctx->width, decoder_ctx->height, AV_PIX_FMT_NV12, 400, 300, AV_PIX_FMT_RGB24, SWS_BICUBIC, NULL, NULL, NULL);
        #endif
    }
    /* read all the output frames (in general there may be any number of them */
    while (ret >= 0) {
        ret = avcodec_receive_frame(dec_ctx, frame);
        if (ret == AVERROR(EAGAIN) || ret == AVERROR_EOF) {
            av_freep(&buffer);
            av_frame_free(&pFrameRGB);
            av_frame_free(&swFrame);

            return;
        }
        else if (ret < 0) {
            fprintf(stderr, "Error during decoding\n");
            goto fail;
        }

        // download frame from gpu to cpu
        ret = av_hwframe_transfer_data(swFrame, frame, 0);
        if (ret < 0) {
            fprintf(stderr, "Error transferring the data to system memory\n");
            goto fail;
        }

        // if (firstTime == 0) {
        //     firstTime = 1;
        //     swsContext = sws_getContext(frame->width, frame->height, AV_PIX_FMT_NV12, frame->width, frame->height, AV_PIX_FMT_RGB24, SWS_BICUBIC, NULL, NULL, NULL);
        // }

        if (swsContext == NULL) {
            printf("swsContext failed.\n");
            goto fail;
        }
        #if !RESCALE
        sws_scale(swsContext, (const unsigned char* const*)swFrame->data, swFrame->linesize, 0, decoder_ctx->height, pFrameRGB->data, pFrameRGB->linesize);
        SaveFrame(pFrameRGB, decoder_ctx->width, decoder_ctx->height, timestamp);
        #else
        sws_scale(swsContext, (const unsigned char* const*)swFrame->data, swFrame->linesize, 0, decoder_ctx->height, pFrameRGB->data, pFrameRGB->linesize);
        SaveFrame(pFrameRGB, 400, 300, timestamp);
        #endif
    }
fail:
    av_freep(&buffer);
    av_free(&pFrameRGB);
    av_free(&swFrame);
    // sws_freeContext(swsContext);
    return;    
}

void ds_feedpkt(char* pktdata, int pktsize, int timestamp){
    AVFrame *decoded_frame = NULL;  
    AVPacket        packet;
    av_init_packet(&packet);

    packet.data = pktdata;
    packet.size = pktsize;

    if (!decoded_frame) {
        if (!(decoded_frame = av_frame_alloc())) {
            fprintf(stderr, "Could not allocate audio frame\n");
            return;
        }
    }   
    decode_feed(decoder_ctx, &packet, decoded_frame, timestamp);
    if (decoded_frame != NULL)
        av_frame_free(&decoded_frame);
    av_packet_unref(&packet);
    return;
}
