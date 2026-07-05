#include "lunasvg_c.h"
#include "lunasvg.h"
#include <stdlib.h>
#include <string.h>

extern "C" {

void* lunasvg_render(const char* data, int data_len, int* out_w, int* out_h) {
    auto document = lunasvg::Document::loadFromData(data, data_len);
    if (!document) {
        return NULL;
    }

    auto bitmap = document->renderToBitmap();
    if (!bitmap.valid()) {
        return NULL;
    }

    int w = bitmap.width();
    int h = bitmap.height();
    int stride = bitmap.stride();
    
    // Allocate a copy of the ARGB32 data to return to Go.
    // The data is in premultiplied ARGB32 format, but we convert it to RGBA.
    // lunasvg 2.3.9 has convertToRGBA()
    bitmap.convertToRGBA();

    void* result = malloc(h * stride);
    if (result) {
        memcpy(result, bitmap.data(), h * stride);
        *out_w = w;
        *out_h = h;
    }
    return result;
}

void lunasvg_free(void* bitmap_data) {
    if (bitmap_data) {
        free(bitmap_data);
    }
}

}
