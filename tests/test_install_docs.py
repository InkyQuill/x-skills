from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def test_readme_documents_github_one_liner() -> None:
    readme = (ROOT / "README.md").read_text(encoding="utf-8")

    assert (
        "curl -fsSL https://raw.githubusercontent.com/InkyQuill/x-skills/main/install.sh | sh"
        in readme
    )


def test_install_script_checks_required_commands() -> None:
    script = (ROOT / "install.sh").read_text(encoding="utf-8")

    assert "need git" in script
    assert "need uv" in script
    assert "uv tool install" in script
