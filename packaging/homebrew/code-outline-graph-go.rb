# Homebrew formula for code-outline-graph-go
#
# To use this formula directly from this repo:
#   brew tap rushikeshsakharleofficial/tap https://github.com/rushikeshsakharleofficial/homebrew-tap
#   brew install code-outline-graph-go
#
# This file is auto-updated by GoReleaser on each release (via homebrew-tap repo).
# Manual template for reference — actual checksums filled by goreleaser.

class CodeOutlineGraphGo < Formula
  desc "Go code indexing and search MCP server — tree-sitter symbols over SQLite FTS5"
  homepage "https://github.com/rushikeshsakharleofficial/gocode-outline-graph"
  license "MIT"
  version "1.1.0"

  on_macos do
    on_arm do
      url "https://github.com/rushikeshsakharleofficial/gocode-outline-graph/releases/download/v#{version}/code-outline-graph-go_#{version}_darwin_arm64.tar.gz"
      # sha256 filled by goreleaser
    end
    on_intel do
      url "https://github.com/rushikeshsakharleofficial/gocode-outline-graph/releases/download/v#{version}/code-outline-graph-go_#{version}_darwin_amd64.tar.gz"
      # sha256 filled by goreleaser
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rushikeshsakharleofficial/gocode-outline-graph/releases/download/v#{version}/code-outline-graph-go_#{version}_linux_arm64.tar.gz"
      # sha256 filled by goreleaser
    end
    on_intel do
      url "https://github.com/rushikeshsakharleofficial/gocode-outline-graph/releases/download/v#{version}/code-outline-graph-go_#{version}_linux_amd64.tar.gz"
      # sha256 filled by goreleaser
    end
  end

  def install
    bin.install "code-outline-graph-go"
  end

  test do
    assert_match "1.1.0", shell_output("#{bin}/code-outline-graph-go version")
  end
end
