set(SOURCES core.cpp util.cpp)
add_library(shop_core STATIC ${SOURCES} ${SHOP_INCLUDE}/shop/core.hpp)
add_library(Shop::core ALIAS shop_core)
add_library(ext::zlib UNKNOWN IMPORTED)
add_executable(shop main.cpp $<$<PLATFORM_ID:Windows>:win.cpp> ${CMAKE_CURRENT_SOURCE_DIR}/util.cpp generated.cpp)
target_sources(shop PRIVATE
  FILE_SET HEADERS BASE_DIRS ${PROJECT_SOURCE_DIR}/include
  FILES ${Shop_SOURCE_DIR}/include/shop/api.hpp)
include(Warnings)
include(${Shop_SOURCE_DIR}/cmake/Warnings.cmake)
shop_warnings(shop)
CPMGetPackage(magic_enum)
add_custom_target(docs ALL COMMAND doxygen SOURCES ../README.md)
