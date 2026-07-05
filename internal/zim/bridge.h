// C-compatible opaque handle types for libzim.
// The actual implementation is in bridge.cpp.
#ifndef ZIM_BRIDGE_H
#define ZIM_BRIDGE_H

#ifdef __cplusplus
extern "C" {
#endif

typedef void* zim_archive_t;
typedef void* zim_entry_t;
typedef void* zim_item_t;

zim_archive_t zim_open(const char* path);
void          zim_close(zim_archive_t archive);
int           zim_get_article_count(zim_archive_t archive);
zim_entry_t   zim_get_main_entry(zim_archive_t archive);
zim_entry_t   zim_get_entry_by_path(zim_archive_t archive, const char* path);
const char*   zim_entry_get_path(zim_entry_t entry);
const char*   zim_entry_get_title(zim_entry_t entry);
zim_item_t    zim_entry_get_item(zim_entry_t entry, int follow);
const char*   zim_item_get_content(zim_item_t item, int* size_out);
const char*   zim_item_get_mimetype(zim_item_t item);
void          zim_entry_free(zim_entry_t entry);
void          zim_item_free(zim_item_t item);

// Batch article listing: iterates iterByTitle() and returns a flat array.
typedef struct { const char* title; const char* path; } zim_article_entry_t;
zim_article_entry_t* zim_list_articles(zim_archive_t archive, int* count_out);
void                 zim_free_article_list(zim_article_entry_t* buf, int count);

#ifdef __cplusplus
}
#endif

#endif
