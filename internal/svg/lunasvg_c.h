#ifndef LUNASVG_C_H
#define LUNASVG_C_H

#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

void* lunasvg_render(const char* data, int data_len, int* out_w, int* out_h);
void lunasvg_free(void* bitmap_data);

#ifdef __cplusplus
}
#endif

#endif
