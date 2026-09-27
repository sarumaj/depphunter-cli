# zsh plugin setup
0=${(%):-%N}
ZSH_PLUGIN_DIR=${0:A:h}
source ${0:A:h}/../scripts/lib/common.sh
source "${ZSH_PLUGIN_DIR}/helpers.zsh"
alias -g G='| grep' gs='git status'
fpath=(${0:A:h}/functions $fpath)
typeset -gx EDITOR=vim
autoload -Uz compinit && compinit
function zsh_greet() { print hi; }
