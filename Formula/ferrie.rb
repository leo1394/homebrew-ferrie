class Ferrie < Formula
  desc "Install Android and iOS apps from files or URLs"
  homepage "https://github.com/leo1394/homebrew-ferrie"
  version "0.2.5"
  license "MIT"

  bottle do
    root_url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.5"
    sha256 cellar: :any_skip_relocation, arm64_sequoia: "fd83f356065003312d5dc2b52b909d2b0ac35b4fe29cbdc0d81b86fd37d005ec"
    sha256 cellar: :any_skip_relocation, sequoia:       "386c8e88c9d446add8f0e837f9565895d7936921709821becdb3b7898d6071bc"
  end

  head do
    url "https://github.com/leo1394/homebrew-ferrie.git", branch: "master"
    depends_on "go" => :build
  end

  on_macos do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.5/ferrie_0.2.5_darwin_arm64", using: :nounzip
      sha256 "40f0fba94a5776db4d2fa87d79091b480053e65445674ec4d1cf1479a4a64783"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.5/ferrie_0.2.5_darwin_amd64", using: :nounzip
      sha256 "ddd0ab5421d96db88ce470b4594d72129150b9cce79ebb055540553d64a583a7"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.5/ferrie_0.2.5_linux_arm64", using: :nounzip
      sha256 "e09fce519db787bdb82c11371a8200a42a9f88e7a6295c204131dc415e9ebbaa"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.5/ferrie_0.2.5_linux_amd64", using: :nounzip
      sha256 "6254cc8019029456083a6403d62f65cd57f0a2f4027b6c0c972f94e8a47f870e"
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
    assert_match "ferrie version 0.2.5 (", output
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
