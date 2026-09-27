!> The shop's front module.
module shop
  use, intrinsic :: iso_fortran_env, only: int64, error_unit
  use iso_c_binding
  use :: shop_cart, only: cart_t, checkout
  use stdlib_kinds, only: dp
  use json_module
  use tomlf, only: toml_table
  use fancy_core
  use mpi_f08
  use netcdf
  use vendor_mod
  use plotter_axes
  use widgets, only: button
  use nothere_mod
  use shop_missing
  use, non_intrinsic :: iso_varying_string
  !$ use omp_lib
  ! use fake_comment
  implicit none
  private
  include 'shop.inc'
  include 'common.inc'
  include 'mpif.h'
  include 'nowhere.inc'

  character(len=*), parameter :: banner = "use fake_string; module fake_mod"
  character(len=*), parameter :: quoted = 'it''s a &
    &use fake_continued'

  type, public :: order_t
    integer :: id
  end type order_t

  interface total
    module procedure total_int, total_real
  end interface total

  abstract interface
    subroutine visitor(x)
      integer, intent(in) :: x
    end subroutine visitor
  end interface

contains

  pure elemental integer function total_int(a) result(t)
    integer, intent(in) :: a
    t = a
  end function total_int

  real(dp) function total_real(a)
    real(dp), intent(in) :: a
    total_real = a
  end function

  recursive subroutine walk(n); integer :: n
    if (n > 0) call walk(n - 1)
  contains
    subroutine inner()
    end subroutine inner
  end subroutine walk

  type(order_t) function new_order(id) result(o)
    integer, intent(in) :: id
    o%id = id
  end function new_order

end module shop
