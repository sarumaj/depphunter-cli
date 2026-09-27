get_filename_component(ROOT ${CMAKE_CURRENT_SOURCE_DIR}/.. ABSOLUTE)
add_executable(shopctl ctl.cpp "${ROOT}/src/util.cpp")
