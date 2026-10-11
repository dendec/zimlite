#include "lunasvg_c.h"
#include "lunasvg.h"
#include <stdlib.h>
#include <string.h>
#include <cmath>

extern "C" {

static void* render_to_bitmap(lunasvg::Document* document, uint32_t width, uint32_t height, int* out_w, int* out_h) {
    // Reject before LunaSVG allocates the native bitmap, not after readback.
    double w_budget = width ? width : std::ceil(document->width());
    double h_budget = height ? height : std::ceil(document->height());
    if (!std::isfinite(w_budget) || !std::isfinite(h_budget) || w_budget <= 0 || h_budget <= 0 ||
        w_budget * h_budget > 1024.0 * 1024.0) {
        return NULL;
    }
    auto bitmap = document->renderToBitmap(width, height);
    if (!bitmap.valid()) {
        return NULL;
    }

    int w = bitmap.width();
    int h = bitmap.height();
    int stride = bitmap.stride();
    bitmap.convertToRGBA();

    void* result = malloc(h * stride);
    if (result) {
        memcpy(result, bitmap.data(), h * stride);
        *out_w = w;
        *out_h = h;
    }
    return result;
}

void* lunasvg_render(const char* data, int data_len, int* out_w, int* out_h) {
    auto document = lunasvg::Document::loadFromData(data, data_len);
    if (!document) {
        return NULL;
    }
    return render_to_bitmap(document.get(), 0, 0, out_w, out_h);
}

void* lunasvg_render_to_size(const char* data, int data_len, int target_w, int target_h, int* out_w, int* out_h) {
    auto document = lunasvg::Document::loadFromData(data, data_len);
    if (!document) {
        return NULL;
    }
    if (target_w <= 0 || target_h <= 0) {
        return NULL;
    }
    return render_to_bitmap(document.get(), static_cast<uint32_t>(target_w), static_cast<uint32_t>(target_h), out_w, out_h);
}

void lunasvg_free(void* bitmap_data) {
    if (bitmap_data) {
        free(bitmap_data);
    }
}

}
