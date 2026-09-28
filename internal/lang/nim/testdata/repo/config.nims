# begin Nimble config (version 2)
when withDir(thisDir(), system.fileExists("nimble.paths")):
  include "nimble.paths"
# end Nimble config

--path:"vendor/lib"
switch("path", thisDir() / "gen")
# --path:"commented"
