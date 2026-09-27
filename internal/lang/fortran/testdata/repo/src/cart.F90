#include "config.h"
#include <petscsys.h>
MODULE Shop_Cart
#ifdef USE_MPI
  USE mpi
#endif
  IMPLICIT NONE
  TYPE :: cart_t
    INTEGER :: n = 0
  CONTAINS
    PROCEDURE :: add
  END TYPE cart_t

  INTERFACE
    MODULE SUBROUTINE checkout(c)
      CLASS(cart_t), INTENT(INOUT) :: c
    END SUBROUTINE checkout
  END INTERFACE

  INTERFACE OPERATOR(+)
    MODULE PROCEDURE merge_carts
  END INTERFACE

  ENUM, BIND(C)
    ENUMERATOR :: red = 1
  END ENUM

CONTAINS

  SUBROUTINE add(self, k)
    CLASS(cart_t), INTENT(INOUT) :: self
    INTEGER, INTENT(IN) :: k
    self%n = self%n + k
  END SUBROUTINE add

  FUNCTION merge_carts(a, b) RESULT(c)
    TYPE(cart_t), INTENT(IN) :: a, b
    TYPE(cart_t) :: c
    c%n = a%n + b%n
  END FUNCTION merge_carts
ENDMODULE Shop_Cart
