class GcsBench < Formula
  desc "Fast, secure document CLI for AI agents backed by Google Cloud Storage"
  homepage "https://github.com/GlideIdentity/doc-cli"
  url "https://github.com/GlideIdentity/doc-cli.git", branch: "initial-setup"
  version "0.1.0"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "-o", bin/"gcs-bench", "./cmd/gcs-bench/"
  end

  test do
    assert_match "GCS encrypted file system", shell_output("#{bin}/gcs-bench 2>&1", 1)
  end
end
