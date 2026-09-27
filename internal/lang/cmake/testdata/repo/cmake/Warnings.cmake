function(shop_warnings target)
  target_compile_options(${target} PRIVATE -Wall -Wextra)
endfunction()

macro(shop_option name)
  option(${name} "" OFF)
endmacro()
