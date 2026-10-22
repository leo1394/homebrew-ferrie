class Ferrie < Formula
  desc "Install Android and iOS apps from files or URLs"
  homepage "https://github.com/leo1394/homebrew-ferrie"
  version "0.2.6"
  license "MIT"

  head do
    url "https://github.com/leo1394/homebrew-ferrie.git", branch: "master"
    depends_on "go" => :build
  end

  on_macos do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.6/ferrie_0.2.6_darwin_arm64", using: :nounzip
      sha256 "f5446c35c984e7a729b79edba5d411e4bb309314d5e59e6733cb41810b226f47"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.6/ferrie_0.2.6_darwin_amd64", using: :nounzip
      sha256 "b0d5c7ad18fbb7d07ef561cbb573ef3dd26721f59da17cdc7c2207a3a171b0bb"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.6/ferrie_0.2.6_linux_arm64", using: :nounzip
      sha256 "c304dd4ad023b860814e8d19cf6e5077b3c0f7be49f1b50649609b367f6a3901"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.6/ferrie_0.2.6_linux_amd64", using: :nounzip
      sha256 "d8c4efba47d9179ef487cab0bde95cc4e876fa397d84161519f5d71bae6c00d4"
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
    assert_match "ferrie version 0.2.6 (", output
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
