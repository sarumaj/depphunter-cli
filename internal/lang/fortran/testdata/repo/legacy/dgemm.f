C     A fixed-form library routine.
C     USE FAKE_FIXED
*     Level 3 BLAS.
      SUBROUTINE DGEMM(TRANSA, TRANSB, M, N, K, ALPHA, A, LDA, B, LDB,
     $                 BETA, C, LDC)
      INCLUDE 'params.inc'
      DOUBLE PRECISION ALPHA, BETA
      CHARACTER*1 TRANSA
   10 CONTINUE
      RETURN
      END
      DOUBLE PRECISION FUNCTION DDOT(N, DX)
      INTEGER N
      DDOT = 0.0D0
      END FUNCTION
      CHARACTER*(*) FUNCTION LABEL(I)
      LABEL = 'SUBROUTINE FAKE'
      END
      BLOCK DATA SHOPINIT
      COMMON /SHOP/ NITEMS
      DATA NITEMS /0/
      END
      MODULE LEGACY_UTIL
      END
d     PRINT *, 'debug'
      PROGRAM LEGACY
      USE LEGACY_UTIL
      USE SHOP_CART
      END
