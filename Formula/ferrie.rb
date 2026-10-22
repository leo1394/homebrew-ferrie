class Ferrie < Formula
  desc "Install Android and iOS apps from files or URLs"
  homepage "https://github.com/leo1394/homebrew-ferrie"
  version "0.2.4"
  license "MIT"

  bottle do
    root_url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.4"
    sha256 cellar: :any_skip_relocation, arm64_sequoia: "60d37a77499ba3b4691ddc841212be82b891b9565d4d30b4d0fa35195aa37c2c"
    sha256 cellar: :any_skip_relocation, sequoia:       "4190cddcdee7fa86ecaf489a72655dd9bdd38c66839da204a04cf3435cf5104b"
  end

  head do
    url "https://github.com/leo1394/homebrew-ferrie.git", branch: "master"
    depends_on "go" => :build
  end

  on_macos do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.4/ferrie_0.2.4_darwin_arm64", using: :nounzip
      sha256 "e6fee897a0a04292004ca181ddb83f0e1ac44073f2f8fd6200025bf2b0bf7831"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.4/ferrie_0.2.4_darwin_amd64", using: :nounzip
      sha256 "3d0a8d8b0191f555dce4fcdb7072d456b96d67bb29f71d0aa317b7f3cb15135d"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.4/ferrie_0.2.4_linux_arm64", using: :nounzip
      sha256 "7505b2f0c2dad537b445adfe5f4b486056b423b44077b332062fa748968aad45"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.4/ferrie_0.2.4_linux_amd64", using: :nounzip
      sha256 "815f5a743cf066a78e72311c5241e7a08b587c18081e530df1c7c277c29b5bdf"
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
    assert_match "ferrie version 0.2.4 (", output
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
