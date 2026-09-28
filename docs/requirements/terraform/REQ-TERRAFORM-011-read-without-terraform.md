---
id: REQ-TERRAFORM-011
title: Terraform read without Terraform
scope: terraform
type: limitation
priority: must
status: implemented
verification:
  - manual
---

## Statement

The plugin **shall not** run Terraform, OpenTofu or Terragrunt, and does not
evaluate expressions: a module source, path or version that is not a literal
(apart from the Terragrunt locals of REQ-TERRAFORM-010) is not resolved,
`count`/`for_each` and conditionals are not considered, a
`find_in_parent_folders` call in a file that other configurations include is
evaluated from that file's own directory (so usually dropped), a provider's
own dependencies are not asked of any registry (a provider depends on none),
and OSV has no Terraform ecosystem and Trivy no Terraform package type, so no
vulnerability is reported for a registry module or a provider; a module
fetched from a git repository on a public forge at a full commit is asked
about by that commit (REQ-FND-026).

## Rationale

depphunter reads repositories statically and never executes their code. The
vendored tree-sitter HCL grammar was measured first on shallow clones of
terraform-aws-modules/terraform-aws-vpc and terraform-aws-eks,
gruntwork-io/terragrunt-infrastructure-live-example and
cloudposse/terraform-aws-components (1984 `.tf` and `.hcl` files): it parsed
every file without an ERROR node but took 3 to 6 ms per file (6.7 s for the
1801 files of terraform-aws-components). HCL's block and attribute structure
is regular, so the plugin reads it with a scanner of its own (strings and
heredocs as templates, comments, one-line blocks, recovery at the next line),
and the whole analysis of terraform-aws-components, every other plugin
included, takes about 0.4 s.

## Acceptance criteria

1. A file with a line the scanner cannot read keeps the blocks before and
   after it.
