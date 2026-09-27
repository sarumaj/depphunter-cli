cmake_minimum_required(VERSION 3.20)
project(physics LANGUAGES Fortran)
add_library(physics src/physics.f90 src/constants.f90)
