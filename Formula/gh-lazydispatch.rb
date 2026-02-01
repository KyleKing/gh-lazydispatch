# typed: false
# frozen_string_literal: true

class GhLazydispatch < Formula
  desc "Interactive GitHub Workflow Dispatcher (standalone or with the GH CLI)"
  homepage "https://github.com/kyleking/gh-lazydispatch"
  license "MIT"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "#{homepage}/releases/download/v#{version}/gh-lazydispatch-darwin-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_ARM64"
    else
      url "#{homepage}/releases/download/v#{version}/gh-lazydispatch-darwin-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "#{homepage}/releases/download/v#{version}/gh-lazydispatch-linux-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_ARM64"
    else
      url "#{homepage}/releases/download/v#{version}/gh-lazydispatch-linux-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_AMD64"
    end
  end

  def install
    binary_name = "gh-lazydispatch-#{OS.kernel_name.downcase}-#{Hardware::CPU.arch}"
    bin.install binary_name => "gh-lazydispatch"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gh-lazydispatch --version")
  end
end
