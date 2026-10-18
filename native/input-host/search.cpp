#include <zim/archive.h>
#include <zim/search.h>
#include <cstdio>
#include <iostream>
#include <memory>
#include <stdexcept>
#include <string>
#include <unistd.h>

static std::string hex(const std::string& value) {
    static const char* digits = "0123456789abcdef";
    std::string encoded;
    encoded.reserve(value.size() * 2);
    for (unsigned char c : value) {
        encoded += digits[c >> 4];
        encoded += digits[c & 15];
    }
    return encoded;
}

int main(int argc, char** argv) {
    // libzim can print diagnostics to stdout (e.g. unsupported stemming).
    // Keep a separate descriptor for the wire protocol and redirect library
    // stdout to stderr before constructing any libzim objects.
    int descriptor = dup(STDOUT_FILENO);
    if (descriptor < 0) return 1;
    std::unique_ptr<FILE, decltype(&fclose)> protocol(fdopen(descriptor, "w"), fclose);
    if (!protocol) { close(descriptor); return 1; }
    if (dup2(STDERR_FILENO, STDOUT_FILENO) < 0) return 1;
    try {
        if (argc != 4) throw std::runtime_error("expected archive, query, offset");
        const std::string query = argv[2];
        if (query.empty() || query.size() > 2048) throw std::runtime_error("invalid query length");
        size_t used = 0;
        int offset = std::stoi(argv[3], &used);
        if (used != std::string(argv[3]).size() || offset < 0 || offset > 10000)
            throw std::runtime_error("invalid page");
        zim::setClusterCacheMaxSize(0);
        zim::Archive archive(argv[1]);
        if (!archive.hasFulltextIndex()) throw std::runtime_error("This archive has no full-text index");
        zim::Searcher searcher(archive);
        auto search = searcher.search(zim::Query(query));
        auto results = search.getResults(offset, 11);
        int count = 0;
        for (auto it = results.begin(); it != results.end() && count < 10; ++it, ++count) {
            auto title = it.getTitle();
            auto path = it.getPath();
            if (title.size() > 8192 || path.size() > 8192) throw std::runtime_error("result is too large");
            fprintf(protocol.get(), "R\t%s\t%s\n", hex(title).c_str(), hex(path).c_str());
        }
        fprintf(protocol.get(), "D\t%d\n", results.size() > 10 ? 1 : 0);
        return fflush(protocol.get()) == 0 ? 0 : 1;
    } catch (const std::exception& error) {
        fprintf(protocol.get(), "E\t%s\n", hex(error.what()).c_str());
        fflush(protocol.get());
        return 1;
    }
}
