package fortran

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// intrinsic maps the modules compilers provide to the fortran-std package they
// are shown as: the standard's intrinsic modules by their own name, OpenMP's and
// OpenACC's by the API.
//
// Implements: REQ-FORTRAN-007
var intrinsic = map[string]string{
	"iso_fortran_env": "iso_fortran_env", "iso_c_binding": "iso_c_binding",
	"ieee_arithmetic": "ieee_arithmetic", "ieee_exceptions": "ieee_exceptions", "ieee_features": "ieee_features",
	"omp_lib": "openmp", "omp_lib_kinds": "openmp", "openacc": "openacc", "openacc_kinds": "openacc",
}

// stdPackage is the fortran-std package of a module used with `use, intrinsic`.
func stdPackage(module string) string {
	if p, ok := intrinsic[module]; ok {
		return p
	}
	return module
}

// libraries maps the modules of C libraries with Fortran bindings to a header the
// cpp plugin knows the library by, so `use mpi` and `#include <mpi.h>` meet on
// one node (REQ-CPP-006).
//
// Implements: REQ-FORTRAN-007
var libraries = map[string]string{
	"mpi": "mpi.h", "mpi_f08": "mpi.h", "pmpi_f08": "mpi.h",
	"hdf5": "hdf5.h", "h5lt": "hdf5.h", "h5ds": "hdf5.h", "h5tb": "hdf5.h",
	"netcdf": "netcdf.h", "netcdf_f03": "netcdf.h", "netcdf4_f03": "netcdf.h", "pnetcdf": "pnetcdf.h",
	"petsc": "petsc.h", "petscsys": "petsc.h", "petscvec": "petsc.h", "petscmat": "petsc.h", "petscksp": "petsc.h", "petscsnes": "petsc.h", "petscts": "petsc.h",
	"fftw3": "fftw3.h", "mkl_dfti": "mkl.h", "mkl_service": "mkl.h", "blas95": "mkl.h", "lapack95": "mkl.h",
	"cudafor": "cuda_runtime.h", "cublas": "cublas_v2.h", "cufft": "cufft.h",
	"hipfort": "hip/hip_runtime.h", "adios2": "adios2.h", "esmf": "ESMC.h",
}

// includeLibraries maps files that Fortran code includes from a library's
// installation to that library's header, for the same meeting.
var includeLibraries = map[string]string{
	"mpif.h": "mpi.h", "mpif-sizeof.h": "mpi.h",
	"fftw3.f03": "fftw3.h", "fftw3.f": "fftw3.h", "fftw3l.f03": "fftw3.h", "fftw3q.f03": "fftw3.h",
	"mkl.fi": "mkl.h", "mkl_dfti.f90": "mkl.h", "omp_lib.h": "", "netcdf.inc": "netcdf.h", "pnetcdf.inc": "pnetcdf.h",
}

// metaLibraries are fpm's metapackages (a dependency written `mpi = "*"`) that
// stand for a library of the system, by header as above.
var metaLibraries = map[string]string{
	"mpi": "mpi.h", "hdf5": "hdf5.h", "netcdf": "netcdf.h", "blas": "blas.h",
}

// knownPackages maps module name prefixes of well-known fpm packages to the
// package: for projects that use them without saying where they come from.
// Longest prefix first; a prefix ending in "_" also matches the bare word.
//
// Implements: REQ-FORTRAN-007
var knownPackages = []struct{ prefix, packageName string }{
	{"stdlib_", "stdlib"},
	{"json_", "json-fortran"},
	{"tomlf_", "toml-f"},
	{"tomlf", "toml-f"},
	{"jonquil_", "jonquil"},
	{"jonquil", "jonquil"},
	{"testdrive", "test-drive"},
	{"test_drive", "test-drive"},
	{"m_cli2", "M_CLI2"},
	{"fpm_", "fpm"},
	{"minpack_", "minpack"},
	{"minpack", "minpack"},
	{"regex_module", "fortran-regex"},
	{"shlex_module", "fortran-shlex"},
	{"forbear", "forbear"},
	{"penf", "PENF"},
	{"face", "FACE"},
	{"flap", "FLAP"},
	{"fiona", "fiona"},
	{"fortran_yaml_c", "fortran-yaml-c"},
	{"fyaml", "fortran-yaml-c"},
	{"datetime_module", "datetime-fortran"},
	{"functional", "functional-fortran"},
	{"fftpack", "fftpack"},
	{"quadpack", "quadpack"},
}

// knownPackage is the package the curated table names for a module, or "".
func knownPackage(module string) string {
	for _, k := range knownPackages {
		if p := strings.TrimSuffix(k.prefix, "_"); module == p || p != k.prefix && strings.HasPrefix(module, k.prefix) {
			return k.packageName
		}
	}
	return ""
}

// stems are the spellings of a package name a module name may start with: the
// name itself and the name without a "fortran" or "-f" affix (json-fortran:
// json, toml-f: toml, fortran-regex: regex), each with - and . as _.
func stems(packageName string) []string {
	n := strings.ToLower(packageName)
	out := []string{n}
	for _, affix := range []string{"fortran-", "fortran_", "f-"} {
		if s := strings.TrimPrefix(n, affix); s != n && s != "" {
			out = append(out, s)
		}
	}
	for _, affix := range []string{"-fortran", "_fortran", "-f", "_f", "-f90", "f90", "-lib", "lib"} {
		if s := strings.TrimSuffix(n, affix); s != n && s != "" {
			out = append(out, s)
		}
	}
	for i, s := range out {
		out[i] = strings.NewReplacer("-", "_", ".", "_").Replace(s)
	}
	return out
}

// spells reports whether a module's name spells package packageName: the name itself,
// or one of its stems followed by "_" (stdlib_kinds, json_module, regex_module),
// or folded the same (test_drive and test-drive), or folded starting with a stem
// of at least four letters (tomlf and toml-f).
//
// Implements: REQ-FORTRAN-007
func spells(module, packageName string) bool {
	if lang.FoldSeparators(module) == lang.FoldSeparators(packageName) {
		return true
	}
	for _, s := range stems(packageName) {
		if len(s) < 2 {
			continue
		}
		if module == s || strings.HasPrefix(module, s+"_") {
			return true
		}
		if f := lang.FoldSeparators(s); len(f) >= 4 && strings.HasPrefix(lang.FoldSeparators(module), f) {
			return true
		}
	}
	return false
}
