program shop_app
  use shop
  implicit none
  outer: do i = 1, 3
    call run()
  end do outer
contains
  subroutine run()
  end subroutine
end program shop_app
