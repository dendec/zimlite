// Console-only libzim FTS benchmark: no SDL, renderer, or title-tree allocation.
#include <zim/archive.h>
#include <zim/search.h>
#include <zim/zim_config.h>
#include <sys/resource.h>
#include <chrono>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <string>
#include <vector>

#ifndef LIBZIM_WITH_XAPIAN
#error "fts-probe requires libzim built with Xapian"
#endif

using Clock = std::chrono::steady_clock;
static double elapsed(Clock::time_point start) {
    return std::chrono::duration<double, std::milli>(Clock::now() - start).count();
}
static std::string json(const std::string& value) {
    std::string out = "\"";
    const char* hex = "0123456789abcdef";
    for (unsigned char c : value) {
        if (c == '"' || c == '\\') { out += '\\'; out += c; }
        else if (c < 32) { out += "\\u00"; out += hex[c >> 4]; out += hex[c & 15]; }
        else out += c;
    }
    return out + '"';
}
static long memoryKB(const char* filename, const std::string& key) {
    std::ifstream in(filename);
    std::string line;
    while (std::getline(in, line)) {
        if (line.compare(0, key.size(), key) == 0) {
            return std::stol(line.substr(key.size()));
        }
    }
    return -1;
}
static void memory() {
    rusage usage{};
    getrusage(RUSAGE_SELF, &usage);
    std::cout << ",\"rss_kb\":" << memoryKB("/proc/self/status", "VmRSS:")
              << ",\"rss_anon_kb\":" << memoryKB("/proc/self/status", "RssAnon:")
              << ",\"rss_file_kb\":" << memoryKB("/proc/self/status", "RssFile:")
              << ",\"swap_kb\":" << memoryKB("/proc/self/status", "VmSwap:")
              << ",\"cluster_cache_bytes\":" << zim::getClusterCacheCurrentSize()
              << ",\"cluster_cache_max_bytes\":" << zim::getClusterCacheMaxSize()
              << ",\"peak_rss_kb\":" << usage.ru_maxrss
              << ",\"available_kb\":" << memoryKB("/proc/meminfo", "MemAvailable:")
              << ",\"major_faults\":" << usage.ru_majflt;
}
static int number(const std::string& value, int min, int max) {
    size_t used = 0;
    int n = std::stoi(value, &used);
    if (used != value.size() || n < min || n > max) throw std::runtime_error("invalid numeric option: " + value);
    return n;
}
int main(int argc, char** argv) {
    std::cout << std::fixed << std::setprecision(3);
    try {
        int limit = 10, offset = 0, repeat = 1, cacheMB = -1;
        bool snippets = false;
        std::vector<std::string> args;
        for (int i = 1; i < argc; ++i) {
            std::string arg = argv[i];
            if (arg == "--snippets") snippets = true;
            else if (arg == "--limit" || arg == "--offset" || arg == "--repeat" || arg == "--cluster-cache-mb") {
                if (++i == argc) throw std::runtime_error("missing option value");
                if (arg == "--limit") limit = number(argv[i], 1, 100);
                if (arg == "--offset") offset = number(argv[i], 0, 10000);
                if (arg == "--repeat") repeat = number(argv[i], 1, 100);
                if (arg == "--cluster-cache-mb") cacheMB = number(argv[i], 0, 512);
            } else args.push_back(arg);
        }
        if (args.size() < 2) {
            std::cerr << "Usage: fts-probe [--snippets] [--limit 1..100] [--offset 0..10000] [--repeat 1..100] [--cluster-cache-mb 0..512] ARCHIVE QUERY [QUERY ...]\n";
            return 2;
        }
        if (cacheMB >= 0) zim::setClusterCacheMaxSize(static_cast<size_t>(cacheMB) * 1024 * 1024);
        auto start = Clock::now();
        zim::Archive archive(args[0]);
        bool indexed = archive.hasFulltextIndex();
        std::cout << "{\"event\":\"archive\",\"path\":" << json(args[0])
                  << ",\"libzim_version\":" << json(LIBZIM_VERSION)
                  << ",\"articles\":" << archive.getArticleCount()
                  << ",\"has_fulltext_index\":" << (indexed ? "true" : "false")
                  << ",\"open_ms\":" << elapsed(start);
        memory(); std::cout << "}\n" << std::flush;
        if (!indexed) throw std::runtime_error("archive has no full-text index");
        start = Clock::now();
        zim::Searcher searcher(archive);
        std::cout << "{\"event\":\"searcher\",\"init_ms\":" << elapsed(start);
        memory(); std::cout << "}\n" << std::flush;
        for (int run = 1; run <= repeat; ++run) {
            for (size_t q = 1; q < args.size(); ++q) {
                std::cout << "{\"event\":\"query_start\",\"run\":" << run
                          << ",\"query\":" << json(args[q]);
                memory(); std::cout << "}\n" << std::flush;
                start = Clock::now();
                auto search = searcher.search(zim::Query(args[q]));
                auto results = search.getResults(offset, limit);
                double searchMS = elapsed(start);
                int rank = offset;
                auto extractStart = Clock::now();
                for (auto it = results.begin(); it != results.end(); ++it) {
                    std::cout << "{\"event\":\"result\",\"run\":" << run
                              << ",\"query\":" << json(args[q]) << ",\"rank\":" << ++rank
                              << ",\"score_percent\":" << it.getScore()
                              << ",\"title\":" << json(it.getTitle()) << ",\"path\":" << json(it.getPath());
                    if (snippets) std::cout << ",\"snippet\":" << json(it.getSnippet());
                    std::cout << "}\n";
                }
                double extractMS = elapsed(extractStart);
                // Estimating the total may perform additional work; time it separately.
                auto estimateStart = Clock::now();
                int estimated = search.getEstimatedMatches();
                double estimateMS = elapsed(estimateStart);
                std::cout << "{\"event\":\"query\",\"run\":" << run << ",\"query\":" << json(args[q])
                          << ",\"offset\":" << offset << ",\"returned\":" << results.size()
                          << ",\"estimated_matches\":" << estimated << ",\"search_ms\":" << searchMS
                          << ",\"extract_ms\":" << extractMS << ",\"estimate_ms\":" << estimateMS
                          << ",\"total_ms\":" << elapsed(start);
                memory(); std::cout << "}\n" << std::flush;
            }
        }
        return 0;
    } catch (const std::exception& error) {
        std::cout << "{\"event\":\"error\",\"message\":" << json(error.what()) << "}\n";
        return 1;
    }
}
