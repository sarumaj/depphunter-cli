#!/bin/sh
set -e
GOLANGCI_VERSION=v1.55.2
pip install --upgrade pip
pip install -r requirements-dev.txt "requests[socks]==2.31.0" 'flask>=2.0'
python3 -m pip install --user black==24.1.0
npm install -g typescript@5.3.3 @angular/cli@^17 yarn
npm i --save-dev ./local-pkg
go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_VERSION}
go install golang.org/x/tools/cmd/goimports@latest
go install golang.org/x/tools/gopls@v0.14.2
go install ./cmd/...
cargo install ripgrep --version 13.0.0
cargo install cargo-edit@0.12
cargo install --git https://github.com/x/y tool
gem install bundler -v 2.5.3
gem install rake:13.1.0 rubocop
sudo apt-get install -y jq
command -v shellcheck >/dev/null || brew install shellcheck
