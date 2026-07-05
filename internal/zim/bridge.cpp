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

const char* zim_get_main_page_redirect(zim_archive_t archive) {
    try {
        auto* a = static_cast<zim::Archive*>(archive);
        auto entry = a->getMainEntry();
        // Follow redirect chain to get the real target.
        entry = entry.getRedirectEntry();
        std::string path = entry.getPath();
        return strdup(path.c_str());
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

const char* zim_entry_get_path(zim_entry_t entry) {
    if (!entry) return nullptr;
    auto* e = static_cast<zim::Entry*>(entry);
    std::string path = e->getPath();
    return strdup(path.c_str());
}

char zim_entry_get_namespace(zim_entry_t entry) {
    if (!entry) return '\0';
    auto* e = static_cast<zim::Entry*>(entry);
    std::string path = e->getPath();
    return path.empty() ? '\0' : path[0];
}

const char* zim_entry_get_title(zim_entry_t entry) {
    if (!entry) return nullptr;
    auto* e = static_cast<zim::Entry*>(entry);
    std::string title = e->getTitle();
    return strdup(title.c_str());
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
    zim::Blob blob = i->getData();
    size_t sz = blob.size();
    if (size_out) *size_out = static_cast<int>(sz);
    if (sz == 0) return nullptr;
    char* data = (char*)malloc(sz);
    if (data) {
        memcpy(data, blob.data(), sz);
    }
    return data;
}

const char* zim_item_get_mimetype(zim_item_t item) {
    if (!item) return nullptr;
    auto* i = static_cast<zim::Item*>(item);
    std::string mime = i->getMimetype();
    return strdup(mime.c_str());
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
