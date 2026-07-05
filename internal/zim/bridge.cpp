// C++ bridge: wraps libzim C++ API into extern "C" functions.
#include "bridge.h"
#include <zim/archive.h>
#include <zim/entry.h>
#include <zim/item.h>
#include <zim/blob.h>
#include <string>
#include <cstring>
#include <cstdlib>

extern "C" {

zim_archive_t zim_open(const char* path) {
    try {
        auto* archive = new zim::Archive(std::string(path));
        return static_cast<zim_archive_t>(archive);
    } catch (...) {
        return nullptr;
    }
}

void zim_close(zim_archive_t archive) {
    if (archive) {
        delete static_cast<zim::Archive*>(archive);
    }
}

int zim_get_article_count(zim_archive_t archive) {
    auto* a = static_cast<zim::Archive*>(archive);
    return static_cast<int>(a->getArticleCount());
}

zim_entry_t zim_get_main_entry(zim_archive_t archive) {
    try {
        auto* a = static_cast<zim::Archive*>(archive);
        auto* entry = new zim::Entry(a->getMainEntry());
        return static_cast<zim_entry_t>(entry);
    } catch (...) {
        return nullptr;
    }
}

zim_entry_t zim_get_entry_by_path(zim_archive_t archive, const char* path) {
    try {
        auto* a = static_cast<zim::Archive*>(archive);
        auto* entry = new zim::Entry(a->getEntryByPath(std::string(path)));
        return static_cast<zim_entry_t>(entry);
    } catch (...) {
        return nullptr;
    }
}

zim_entry_t zim_get_entry_by_title_index(zim_archive_t archive, int idx) {
    try {
        auto* a = static_cast<zim::Archive*>(archive);
        auto* entry = new zim::Entry(a->getEntryByTitle(static_cast<zim::entry_index_type>(idx)));
        return static_cast<zim_entry_t>(entry);
    } catch (...) {
        return nullptr;
    }
}

const char* zim_entry_get_path(zim_entry_t entry) {
    if (!entry) return nullptr;
    auto* e = static_cast<zim::Entry*>(entry);
    // Return a static copy — caller must not free.
    static thread_local std::string path;
    path = e->getPath();
    return path.c_str();
}

const char* zim_entry_get_title(zim_entry_t entry) {
    if (!entry) return nullptr;
    auto* e = static_cast<zim::Entry*>(entry);
    static thread_local std::string title;
    title = e->getTitle();
    return title.c_str();
}

zim_item_t zim_entry_get_item(zim_entry_t entry, int follow) {
    if (!entry) return nullptr;
    try {
        auto* e = static_cast<zim::Entry*>(entry);
        auto* item = new zim::Item(e->getItem(follow != 0));
        return static_cast<zim_item_t>(item);
    } catch (...) {
        return nullptr;
    }
}

const char* zim_item_get_content(zim_item_t item, int* size_out) {
    if (!item) return nullptr;
    auto* i = static_cast<zim::Item*>(item);
    static thread_local std::string content;
    zim::Blob blob = i->getData();
    content.assign(blob.data(), blob.size());
    if (size_out) *size_out = static_cast<int>(blob.size());
    return content.data();
}

const char* zim_item_get_mimetype(zim_item_t item) {
    if (!item) return nullptr;
    auto* i = static_cast<zim::Item*>(item);
    static thread_local std::string mime;
    mime = i->getMimetype();
    return mime.c_str();
}

void zim_entry_free(zim_entry_t entry) {
    delete static_cast<zim::Entry*>(entry);
}

void zim_item_free(zim_item_t item) {
    delete static_cast<zim::Item*>(item);
}

zim_article_entry_t* zim_list_articles(zim_archive_t archive, int* count_out) {
    auto* a = static_cast<zim::Archive*>(archive);
    auto range = a->iterByTitle();
    int count = static_cast<int>(a->getArticleCount());
    if (count <= 0) {
        *count_out = 0;
        return nullptr;
    }
    auto* buf = new zim_article_entry_t[count];
    int i = 0;
    for (auto& entry : range) {
        buf[i].title = strdup(entry.getTitle().c_str());
        buf[i].path = strdup(entry.getPath().c_str());
        if (++i >= count) break;
    }
    *count_out = i;
    return buf;
}

void zim_free_article_list(zim_article_entry_t* buf, int count) {
    if (buf == nullptr) return;
    for (int i = 0; i < count; i++) {
        free((void*)buf[i].title);
        free((void*)buf[i].path);
    }
    delete[] buf;
}

} // extern "C"
