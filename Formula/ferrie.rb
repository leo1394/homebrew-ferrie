class Ferrie < Formula
  desc "Install Android and iOS apps from files or URLs"
  homepage "https://github.com/leo1394/homebrew-ferrie"
  version "0.2.8"
  license "MIT"

  bottle do
    root_url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.8"
    sha256 cellar: :any_skip_relocation, arm64_sequoia: "cfff469a842840fc21e2ef33532a9ba5247288d8ed3ab4186fe1ef41e6bf4c7a"
    sha256 cellar: :any_skip_relocation, sequoia:       "c6e6395a9ae5a384408eacc35bc7034c7a2784270b11b11c933f40b4e869084d"
  end

  head do
    url "https://github.com/leo1394/homebrew-ferrie.git", branch: "master"
    depends_on "go" => :build
  end

  on_macos do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.8/ferrie_0.2.8_darwin_arm64", using: :nounzip
      sha256 "565cbaee44bcc26e3110953b2bfe613357082d76df76a1d1ccf77cb1c83aaf52"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.8/ferrie_0.2.8_darwin_amd64", using: :nounzip
      sha256 "e29aeaa2b257a551a0f8e41ddf52a7894c792dcd5b68c1e2b978ea17b77d148e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.8/ferrie_0.2.8_linux_arm64", using: :nounzip
      sha256 "2979367b054cddabe35ba24fe12cfea00071eb5314f0529096aa2a94649ef4bb"
    end
    on_intel do
      url "https://github.com/leo1394/homebrew-ferrie/releases/download/v0.2.8/ferrie_0.2.8_linux_amd64", using: :nounzip
      sha256 "0dd196c8e524544546aff865b60c4a4234f891fa5d9901a1130f56e1ca46b9bf"
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
    assert_match "ferrie version 0.2.8 (", output
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
