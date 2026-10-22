class Ferrie < Formula
  desc "Install Android and iOS apps from files or URLs"
  homepage "https://github.com/leo1394/homebrew-ferrie"
  version "0.2.7"
  license "MIT"

  bottle do
    root_url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.7"
    sha256 cellar: :any_skip_relocation, arm64_sequoia: "ec9f1312aed5b696ecb985535915fde0f6d133fe552f94c3d4dff50a78a152a2"
    sha256 cellar: :any_skip_relocation, sequoia:       "89533fb7eca9a9a0a623df94d914768fb01917cc78df57d87b1ba28501c4a4b9"
  end

  head do
    url "https://github.com/leo1394/homebrew-ferrie.git", branch: "master"
    depends_on "go" => :build
  end

  on_macos do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.7/ferrie_0.2.7_darwin_arm64", using: :nounzip
      sha256 "54ab5a342d83e0b9c214f4e4339edce8608fa50d0b5a94c956a6531ef7fd8247"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.7/ferrie_0.2.7_darwin_amd64", using: :nounzip
      sha256 "9db09085e52bd65136caba337e503a3bfff47a2621a2e80307cc9c05c65b8f5b"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.7/ferrie_0.2.7_linux_arm64", using: :nounzip
      sha256 "796db932ac5ccee1614cc9053511065a69a84cf091e23a9a32bcc768787a17ff"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.7/ferrie_0.2.7_linux_amd64", using: :nounzip
      sha256 "4859f49640e100aa9579262f9cb207d110b7c666fa3ca49dd931c854440cb4fb"
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
    assert_match "ferrie version 0.2.7 (", output
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
