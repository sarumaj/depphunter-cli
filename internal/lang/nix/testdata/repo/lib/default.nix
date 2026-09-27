let
  helper = x: x;
  version = builtins.readFile ./VERSION;
in
{
  inherit helper version;
  mkThing = { name }: name;
  strings.upper = s: s;
  "quoted" = 1;
  ${helper "dyn"} = 2;
}
