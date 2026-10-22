class Ferrie < Formula
  desc "Install Android and iOS apps from files or URLs"
  homepage "https://github.com/leo1394/homebrew-ferrie"
  version "0.2.3"
  license "MIT"

  head do
    url "https://github.com/leo1394/homebrew-ferrie.git", branch: "master"
    depends_on "go" => :build
  end

  on_macos do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.3/ferrie_0.2.3_darwin_arm64", using: :nounzip
      sha256 "d8ab2797ad568de1e85030e8a5299c4bbc044ea7440214b25c8e640559d28e76"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.3/ferrie_0.2.3_darwin_amd64", using: :nounzip
      sha256 "5ee7748adcff42240bf13093a8b46d97efa13d33bd966035f2a9cbef2bebe311"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.3/ferrie_0.2.3_linux_arm64", using: :nounzip
      sha256 "59e9fa7d784016fbd8a0655a902a6444d60004f9ec69c8bd493f9ae499c6563d"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.3/ferrie_0.2.3_linux_amd64", using: :nounzip
      sha256 "5cd38c2dcd457a953b1c58d7428a14e8ca57d2ea37955b3d2c1a14db9b47e9d2"
    end
  end

  def install
    if build.head?
      system "go", "build", *std_go_args(ldflags: "-s -w"), "."
    else
      bin.install Dir["ferrie_*"][0] => "ferrie"
    end
    chmod 0755, bin/"ferrie"
    generate_completions_from_executable(bin/"ferrie", "__completion")
    man1.mkpath
    (man1/"ferrie.1").write Utils.safe_popen_read(bin/"ferrie", "__man")
    pwsh_completion.mkpath
    (pwsh_completion/"ferrie.ps1").write Utils.safe_popen_read(bin/"ferrie", "__completion", "powershell")
  end

  test do
    output = shell_output("#{bin}/ferrie --version")
    assert_match "ferrie version 0.2.3 (", output
    assert_match(%r{\(\d{4}-\d{2}-\d{2}\)\nhttps://github.com/leo1394/homebrew-ferrie\n\z}, output)
    assert_match "--target", shell_output("#{bin}/ferrie --help")
    assert_match "Did you mean '--target'", shell_output("#{bin}/ferrie --targte app.apk 2>&1", 2)
    assert_match "FERRIE", (man1/"ferrie.1").read
    assert_path_exists bash_completion/"ferrie"
    assert_path_exists zsh_completion/"_ferrie"
    assert_path_exists fish_completion/"ferrie.fish"
    assert_path_exists pwsh_completion/"ferrie.ps1"
  end
end
