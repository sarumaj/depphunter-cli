(defsystem "acme"
  :class :package-inferred-system
  :pathname "src"
  :depends-on ("acme/main"))

(asdf:register-system-packages "closer-mop" '(:c2mop))
