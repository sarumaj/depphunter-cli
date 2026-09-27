# Third-party content fetched at configure time.
FetchContent_Declare(
  googletest
  GIT_REPOSITORY https://github.com/google/googletest.git
  GIT_TAG        f8d7d77c06936315286eb55f8de22cd23c188571 # v1.14.0
)
FetchContent_Declare(Catch2
  GIT_REPOSITORY https://github.com/catchorg/Catch2.git
  GIT_TAG v3.5.2)
FetchContent_Declare(json
  URL https://github.com/nlohmann/json/releases/download/v3.11.3/json.tar.xz
  URL_HASH SHA256=d6c65aca6b1ed68e7a182f4757257b107ae403032760ed6ef121c9d55e81757d)
FetchContent_Declare(spdlog GIT_REPOSITORY git@github.com:gabime/spdlog.git GIT_TAG origin/main)
FetchContent_Declare(cli11 GIT_REPOSITORY "https://github.com/CLIUtils/CLI11")
FetchContent_Declare(zip URL [[https://example.com/downloads/zip-1.0.tar.gz]])
ExternalProject_Add(legacy URL ${CMAKE_CURRENT_LIST_DIR}/../third_party/legacy-1.2.tar.gz)

set(CPM_DOWNLOAD_VERSION 0.38.7)
include(CPM)
CPMAddPackage("gh:TartanLlama/expected@1.1.0")
CPMAddPackage(NAME magic_enum GITHUB_REPOSITORY Neargye/magic_enum VERSION 0.9.5)
CPMAddPackage(
  NAME doctest
  GIT_REPOSITORY https://github.com/doctest/doctest.git
  GIT_TAG ae7a13539fb71f270b87eb2e874fbac80bc8dda2)

function(shop_fetch name)
  FetchContent_Declare(${name} GIT_REPOSITORY https://git.example.com/${name}.git)
endfunction()

FetchContent_MakeAvailable(googletest Catch2 json unknown)
