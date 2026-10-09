"""Validate fork releases and registry identities using only Python's standard library."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import urllib.error
import urllib.request

REPOSITORY = "cesar-carlos/evolution-go"
UPSTREAM = "evolution-foundation/evolution-go"
IMAGE = "ghcr.io/" + REPOSITORY
VERSION_PATTERN = r"(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)-cesar\.[1-9][0-9]*"
BASE_ANNOTATION = "io.github.cesar-carlos.upstream.revision"


def run(*args):
    return subprocess.check_output(args, text=True).strip()


def read_base(root=Path(".")):
    base = json.loads((root / ".github/upstream-base.json").read_text(encoding="utf-8"))
    if base.get("repository") != UPSTREAM:
        raise ValueError("Unexpected upstream repository")
    if not re.fullmatch(r"v?(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", base.get("tag", "")):
        raise ValueError("Upstream base must be a stable version tag")
    if not re.fullmatch(r"[0-9a-f]{40}", base.get("sha", "")):
        raise ValueError("Upstream base requires a full commit SHA")
    return base


def validate_version(version, tag, base):
    if not re.fullmatch(VERSION_PATTERN, version):
        raise ValueError("VERSION must use X.Y.Z-cesar.N (N >= 1)")
    if tag != "v" + version:
        raise ValueError("Tag does not match VERSION")
    if version.split("-cesar.")[0] != base["tag"].removeprefix("v"):
        raise ValueError("Fork version must identify the recorded upstream version")


def expected_annotations(version, commit, base):
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("Image identity requires a full commit SHA")
    return {
        "org.opencontainers.image.source": "https://github.com/" + REPOSITORY,
        "org.opencontainers.image.revision": commit,
        "org.opencontainers.image.version": version,
        BASE_ANNOTATION: base["sha"],
    }


def validate_checkout(commit, base):
    if run("git", "rev-parse", "HEAD") != commit:
        raise ValueError("Checked out commit differs from the requested release")
    subprocess.check_call(["git", "cat-file", "-e", base["sha"] + "^{commit}"])
    subprocess.check_call(["git", "merge-base", "--is-ancestor", base["sha"], commit])
    subprocess.check_call(["git", "merge-base", "--is-ancestor", commit, "refs/remotes/origin/main"])


def verify_manifest(manifest, version, commit, base):
    for key, value in expected_annotations(version, commit, base).items():
        if manifest.get("annotations", {}).get(key) != value:
            raise ValueError("Existing image has unexpected identity: " + key)
    platforms = [m.get("platform", {}) for m in manifest.get("manifests", [])]
    platforms = [(p.get("os"), p.get("architecture")) for p in platforms if p.get("os") != "unknown"]
    if sorted(platforms) != [("linux", "amd64"), ("linux", "arm64")]:
        raise ValueError("Image must contain exactly linux/amd64 and linux/arm64")


def inspect_image(reference):
    result = subprocess.run(
        ["docker", "buildx", "imagetools", "inspect", "--raw", reference],
        text=True, capture_output=True,
    )
    if result.returncode:
        # Authentication, networking and registry failures must never authorize an overwrite.
        error = result.stderr.lower()
        if "unauthorized" not in error and "denied" not in error and any(
            marker in error for marker in ("manifest unknown", "manifest_unknown", "not found")
        ):
            return None
        raise ValueError("Registry inspection failed: " + result.stderr.strip())
    manifest = json.loads(result.stdout)
    descriptor = json.loads(run("docker", "buildx", "imagetools", "inspect", reference,
                                "--format", "{{json .Manifest}}"))
    digest = descriptor.get("digest", "")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
        raise ValueError("Registry returned an invalid digest")
    return manifest, digest


def registry_state(version, commit, base):
    found = []
    for tag in (version, "sha-" + commit):
        result = inspect_image(IMAGE + ":" + tag)
        if result is not None:
            manifest, digest = result
            verify_manifest(manifest, version, commit, base)
            found.append(digest)
    if len(set(found)) > 1:
        raise ValueError("Version and SHA tags identify different images")
    return found[0] if found else ""


def package_exists():
    # A new GHCR namespace can deny manifest reads until the first push. Confirm
    # package absence via GitHub instead of interpreting that denial as a free tag.
    token = os.environ.get("GH_TOKEN")
    if not token:
        return True  # Local inspection uses the normal registry guards.
    request = urllib.request.Request(
        "https://api.github.com/users/cesar-carlos/packages/container/evolution-go",
        headers={"Authorization": "Bearer " + token, "Accept": "application/vnd.github+json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=30):
            return True
    except urllib.error.HTTPError as error:
        if error.code == 404:
            return False
        raise ValueError("Cannot verify package availability (HTTP " + str(error.code) + ")") from error


def output(values):
    for key, value in values.items():
        print(key + "=" + value)
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as handle:
            for key, value in values.items():
                handle.write(key + "=" + value + "\n")


def verify_release_notes(existing, expected):
    if not isinstance(existing, str):
        raise ValueError("Existing release must contain a text body")
    # GitHub/CLI line endings do not change the published Markdown content.
    normalize = lambda value: value.replace("\r\n", "\n").rstrip("\n")
    if normalize(existing) != normalize(expected):
        raise ValueError("Existing release notes differ from the verified publication")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("validate", "fetch-base", "state", "verify", "notes", "compare-notes"))
    parser.add_argument("--tag")
    parser.add_argument("--commit")
    parser.add_argument("--digest")
    parser.add_argument("--existing-notes")
    parser.add_argument("--expected-notes")
    args = parser.parse_args()
    if args.command == "compare-notes":
        if not args.existing_notes or not args.expected_notes:
            raise ValueError("Notes comparison requires both existing JSON and expected Markdown")
        existing = json.loads(Path(args.existing_notes).read_text(encoding="utf-8"))
        verify_release_notes(existing.get("body"), Path(args.expected_notes).read_text(encoding="utf-8"))
        return
    base = read_base()
    version = Path("VERSION").read_text(encoding="utf-8").strip()

    if args.command == "fetch-base":
        subprocess.check_call(["git", "fetch", "--no-tags", "https://github.com/" + UPSTREAM + ".git",
                               "refs/tags/" + base["tag"] + ":refs/tags/fork-upstream-base"])
        if run("git", "rev-parse", "refs/tags/fork-upstream-base^{commit}") != base["sha"]:
            raise ValueError("Official tag differs from the recorded upstream SHA")
        output({"base": base["sha"]})
        return

    validate_version(version, args.tag or "v" + version, base)
    commit = args.commit or run("git", "rev-parse", "HEAD")
    expected_annotations(version, commit, base)
    if args.command == "validate":
        if os.environ.get("GITHUB_REPOSITORY", REPOSITORY) != REPOSITORY:
            raise ValueError("Publishing is restricted to the maintained fork")
        validate_checkout(commit, base)
        output({"version": version, "commit": commit, "base": base["sha"], "image": IMAGE})
    elif args.command == "state":
        output({"digest": registry_state(version, commit, base) if package_exists() else ""})
    elif args.command == "verify":
        if not args.digest or not re.fullmatch(r"sha256:[0-9a-f]{64}", args.digest):
            raise ValueError("Verification requires an immutable digest")
        result = inspect_image(IMAGE + "@" + args.digest)
        if result is None or result[1] != args.digest:
            raise ValueError("Published image digest is unavailable")
        verify_manifest(result[0], version, commit, base)
        # Also enforce that both published aliases resolve to this exact image.
        for tag in (version, "sha-" + commit):
            alias = inspect_image(IMAGE + ":" + tag)
            if alias is None or alias[1] != args.digest:
                raise ValueError("Published alias differs from the verified digest: " + tag)
    elif args.command == "notes":
        if not args.digest or not re.fullmatch(r"sha256:[0-9a-f]{64}", args.digest):
            raise ValueError("Release notes require a verified image digest")
        url = "https://github.com/" + REPOSITORY + "/blob/" + commit
        print("# " + version + " — distribuição independente do fork\n")
        print("Base oficial: [" + base["tag"] + "](https://github.com/" + UPSTREAM + "/commit/" + base["sha"] + ").")
        print("\nCommit do fork: `" + commit + "`. Arquiteturas: `linux/amd64`, `linux/arm64`.\n")
        print("## Instalação reproduzível\n\n```sh\ndocker pull " + IMAGE + "@" + args.digest + "\n```\n")
        print("Imagem: `" + IMAGE + ":" + version + "`\n\nDigest: `" + args.digest + "`\n")
        print("## Correções e manutenção\n")
        for line in Path("docs/fork/PATCHES.md").read_text(encoding="utf-8").splitlines():
            if line.startswith("## F-"):
                print("- " + line.removeprefix("## "))
        print("\n[Inventário e evidências](" + url + "/docs/fork/PATCHES.md).")
        print("\n[Atualização, homologação e reversão](" + url + "/docs/fork/MAINTENANCE.md).")
        print("\n## Limitações\n\nCompatibilidade de listas/botões foi adaptada e segue pendente de homologação Android/iOS/Web. "
              "A CI não envia mensagens reais e não comprova renderização em aparelhos. "
              "Pareamento, reconexão e passkey reais continuam pendentes de homologação. Integrações com WhatsApp/licença/serviços reais exigem homologação separada. "
              "Esta release não implanta em produção. Para reverter, use o digest anterior registrado "
              "e confira a compatibilidade de banco e sessões antes de voltar a imagem.")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print("Release rejected: " + str(error), file=sys.stderr)
        sys.exit(1)
