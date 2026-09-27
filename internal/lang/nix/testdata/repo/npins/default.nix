# npins' generated loader (abridged)
builtins.mapAttrs (name: pin: pin) (builtins.fromJSON (builtins.readFile ./sources.json)).pins
