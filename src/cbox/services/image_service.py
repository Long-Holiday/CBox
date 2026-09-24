"""Image parsing, building, and remote runtime materialization service."""

import os
import shlex
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional
from sqlalchemy.orm import Session

from cbox.domain.image import CopyInstruction, ImageManifest, ImageSummary
from cbox.storage.repositories import ImageRepository, RuntimeRepository
from cbox.transport.ssh import SSHTransport
from cbox.utils.errors import ImageBuildError, ImageMaterializationError, NotFoundError
from cbox.utils.hash import canonical_json_hash, sha256_file, short_id
from cbox.utils.paths import default_paths


class ImageService:
    """Manages Cboxfile parsing, image registration, and remote runtime materialization."""

    def __init__(self, session: Session):
        self.session = session
        self.repo = ImageRepository(session)
        self.runtime_repo = RuntimeRepository(session)

    @staticmethod
    def parse_cboxfile(cboxfile_path: Path | str, context_dir: Optional[Path | str] = None) -> ImageManifest:
        """Parse Cboxfile syntax into an ImageManifest."""
        path = Path(cboxfile_path).resolve()
        if not path.is_file():
            raise ImageBuildError(f"Cboxfile not found: {path}")

        ctx_dir = Path(context_dir).resolve() if context_dir else path.parent

        manifest = ImageManifest(
            id="",
            tags=[],
            base={"provider": "colab", "python": "3"},
            apt=[],
            pip=[],
            environment={},
            workdir="/workspace",
            copies=[],
            run_commands=[],
            cmd=["bash"],
            healthcheck=None,
        )

        with open(path, "r", encoding="utf-8") as f:
            lines = f.readlines()

        for idx, raw_line in enumerate(lines, 1):
            line = raw_line.strip()
            if not line or line.startswith("#"):
                continue

            parts = line.split(None, 1)
            directive = parts[0].upper()
            arg = parts[1].strip() if len(parts) > 1 else ""

            if directive == "FROM":
                # e.g. FROM colab/python:3
                manifest.base["image"] = arg
            elif directive == "APT":
                manifest.apt.extend(arg.split())
            elif directive == "PIP":
                manifest.pip.extend(arg.split())
            elif directive == "ENV":
                # e.g. ENV PYTHONPATH=/workspace
                if "=" in arg:
                    k, v = arg.split("=", 1)
                    manifest.environment[k.strip()] = v.strip().strip("\"'")
                else:
                    k_v = arg.split(None, 1)
                    if len(k_v) == 2:
                        manifest.environment[k_v[0]] = k_v[1].strip("\"'")
            elif directive == "WORKDIR":
                manifest.workdir = arg
            elif directive == "COPY":
                copy_parts = arg.split()
                if len(copy_parts) >= 2:
                    src = copy_parts[0]
                    dest = copy_parts[1]
                    manifest.copies.append(CopyInstruction(src=src, dest=dest))
            elif directive == "RUN":
                manifest.run_commands.append(arg)
            elif directive == "CMD":
                if arg.startswith("[") and arg.endswith("]"):
                    import json, ast
                    try:
                        manifest.cmd = json.loads(arg)
                    except Exception:
                        try:
                            manifest.cmd = list(ast.literal_eval(arg))
                        except Exception:
                            manifest.cmd = shlex.split(arg.strip("[]"))
                else:
                    manifest.cmd = shlex.split(arg)
            elif directive == "HEALTHCHECK":
                manifest.healthcheck = arg
            else:
                raise ImageBuildError(f"Line {idx}: Unsupported Cboxfile directive '{directive}'")

        # Compute deterministic ID
        canonical_data = manifest.to_canonical_dict()

        # Include hashes of copied local files
        file_hashes = {}
        for c in manifest.copies:
            local_src = (ctx_dir / c.src).resolve()
            if local_src.exists():
                file_hashes[c.src] = sha256_file(local_src) if local_src.is_file() else "dir"
            else:
                raise ImageBuildError(f"COPY source file does not exist: {local_src}")
        canonical_data["copied_file_hashes"] = file_hashes

        image_hash = canonical_json_hash(canonical_data)
        manifest.id = f"sha256:{image_hash}"
        return manifest

    def build_image(
        self,
        context_dir: Path | str,
        tag: Optional[str] = None,
        cboxfile_name: str = "Cboxfile",
    ) -> ImageManifest:
        """Build image from Cboxfile in context_dir, persist metadata, and register tag."""
        ctx_path = Path(context_dir).resolve()
        cboxfile = ctx_path / cboxfile_name
        manifest = self.parse_cboxfile(cboxfile, context_dir=ctx_path)
        if tag:
            if tag not in manifest.tags:
                manifest.tags.append(tag)

        # Store image bundle in local cbox storage
        img_storage_dir = default_paths.images_dir / short_id(manifest.id)
        img_storage_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
        (img_storage_dir / "manifest.json").write_text(manifest.model_dump_json(indent=2), encoding="utf-8")

        # Save to database
        self.repo.save_image(manifest)
        return manifest

    def get_image(self, identifier: str) -> ImageManifest:
        img = self.repo.get_by_id_or_tag(identifier)
        if not img:
            raise NotFoundError(f"Image not found: {identifier}")
        return img

    def list_images(self) -> list[ImageSummary]:
        return self.repo.list_images()

    def remove_image(self, identifier: str) -> bool:
        img = self.repo.get_by_id_or_tag(identifier)
        if not img:
            return False
        img_dir = default_paths.images_dir / short_id(img.id)
        if img_dir.exists():
            import shutil
            shutil.rmtree(img_dir, ignore_errors=True)
        return self.repo.delete_image(identifier)

    async def materialize_image(
        self,
        runtime_id: str,
        image_identifier: str,
        transport: SSHTransport,
        context_dir: Optional[Path | str] = None,
    ) -> bool:
        """Materialize image environment blueprint on remote runtime.

        Checks image cache marker. If cached, skips re-installation.
        """
        manifest = self.get_image(image_identifier)
        cache_marker = f"/content/.cbox/cache/images/{short_id(manifest.id)}.ready"

        # Check remote marker
        check_res = await transport.exec(["test", "-f", cache_marker])
        if check_res.returncode == 0:
            # Cache HIT!
            self.runtime_repo.add_cache_entry(runtime_id, "image", manifest.id, manifest.id)
            return False  # False means skipped (cache hit)

        # Cache MISS: materialize blueprint
        # 1. APT packages
        if manifest.apt:
            apt_pkgs = " ".join(shlex.quote(p) for p in manifest.apt)
            res = await transport.exec(
                ["sh", "-c", f"DEBIAN_FRONTEND=noninteractive apt-get update -qq && apt-get install -y -qq {apt_pkgs}"],
                timeout=180.0,
            )
            if not res.success:
                raise ImageMaterializationError(f"Failed to install APT packages: {res.stderr}")

        # 2. PIP packages
        if manifest.pip:
            pip_pkgs = " ".join(shlex.quote(p) for p in manifest.pip)
            res = await transport.exec(
                ["sh", "-c", f"pip install -q {pip_pkgs}"],
                timeout=300.0,
            )
            if not res.success:
                raise ImageMaterializationError(f"Failed to install PIP packages: {res.stderr}")

        # 3. COPY files
        if manifest.copies and context_dir:
            ctx_path = Path(context_dir).resolve()
            for copy_item in manifest.copies:
                src_file = ctx_path / copy_item.src
                if src_file.is_file():
                    await transport.upload_text(src_file.read_text(encoding="utf-8", errors="replace"), copy_item.dest)

        # 4. RUN instructions
        for run_cmd in manifest.run_commands:
            res = await transport.exec(["sh", "-c", run_cmd], timeout=300.0)
            if not res.success:
                raise ImageMaterializationError(f"Failed to execute RUN command '{run_cmd}': {res.stderr}")

        # 5. Write cache marker
        await transport.exec(["mkdir", "-p", "/content/.cbox/cache/images"])
        await transport.upload_text(datetime.now(timezone.utc).isoformat(), cache_marker)
        self.runtime_repo.add_cache_entry(runtime_id, "image", manifest.id, manifest.id)
        return True  # Materialized
