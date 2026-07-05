// C++ bridge: wraps libzim C++ API into extern "C" functions.
#include "bridge.h"
#include <zim/archive.h>
#include <zim/entry.h>
#include <zim/item.h>
#include <zim/blob.h>
#include <string>
#include <cstring>

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

} // extern "C"
