"""Volume creation, caching, and bidirectional synchronization service."""

import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional
from sqlalchemy.orm import Session

from cbox.domain.enums import VolumeMode
from cbox.domain.volume import VolumeManifest, VolumeMount, VolumeSummary
from cbox.storage.repositories import RuntimeRepository, VolumeRepository
from cbox.transport.proxy import TarStreamTransport, decide_transfer_mode
from cbox.transport.rsync import RsyncTransport
from cbox.transport.ssh import SSHTransport
from cbox.utils.errors import NotFoundError, VolumeSyncError
from cbox.utils.hash import compute_directory_checksum, compute_directory_fast_hash, sha256_text, short_id
from cbox.utils.paths import default_paths


class VolumeService:
    """Manages Volume manifests, caching checks, and sync between host and remote runtimes."""

    def __init__(self, session: Session):
        self.session = session
        self.repo = VolumeRepository(session)
        self.runtime_repo = RuntimeRepository(session)

    def create_volume(
        self,
        name: str,
        source: Path | str,
        mode: VolumeMode = VolumeMode.RO,
        immutable: bool = False,
        use_checksum: bool = False,
    ) -> VolumeManifest:
        """Register a managed named volume with hash computation."""
        src_path = Path(source).resolve()
        if not src_path.exists():
            # If output volume source doesn't exist, create it
            if mode == VolumeMode.OUTPUT:
                src_path.mkdir(parents=True, exist_ok=True, mode=0o755)
            else:
                raise VolumeSyncError(f"Volume source path does not exist: {src_path}")

        vol_hash = (
            compute_directory_checksum(src_path)
            if use_checksum
            else compute_directory_fast_hash(src_path)
        )

        vol_id = f"vol-{short_id(sha256_text(name + str(src_path)))}"
        manifest = VolumeManifest(
            id=vol_id,
            name=name,
            source=str(src_path),
            hash=vol_hash,
            mode=mode,
            immutable=immutable,
            version=1,
        )
        self.repo.save_volume(manifest)
        return manifest

    def get_volume(self, identifier: str) -> VolumeManifest:
        vol = self.repo.get_by_name_or_id(identifier)
        if not vol:
            raise NotFoundError(f"Volume not found: {identifier}")
        return vol

    def list_volumes(self) -> list[VolumeSummary]:
        return self.repo.list_volumes()

    def remove_volume(self, identifier: str) -> bool:
        return self.repo.delete_volume(identifier)

    async def sync_mount_to_runtime(
        self,
        mount: VolumeMount,
        runtime_id: str,
        transport: SSHTransport,
        transfer_mode_pref: str = "auto",
    ) -> bool:
        """Synchronize volume data to remote runtime.

        Returns True if data was transferred, False if skipped due to cache hit.
        """
        # Resolve volume name or anonymous directory path
        vol = self.repo.get_by_name_or_id(mount.source)
        if vol:
            source_path = Path(vol.source)
            vol_key = vol.name
            mode = mount.mode or vol.mode
            immutable = vol.immutable
        else:
            source_path = Path(mount.source).resolve()
            vol_key = short_id(sha256_text(str(source_path)))
            mode = mount.mode
            immutable = False

        if not source_path.exists():
            if mode == VolumeMode.OUTPUT:
                source_path.mkdir(parents=True, exist_ok=True, mode=0o755)
            else:
                raise VolumeSyncError(f"Mount source does not exist: {source_path}")

        # Compute hash
        curr_hash = compute_directory_fast_hash(source_path)
        remote_vol_dir = f"/content/.cbox/volumes/{mount.source}"
        remote_manifest_file = f"{remote_vol_dir}/.cbox_manifest.json"

        # Check if remote already has identical hash
        check_res = await transport.exec(["cat", remote_manifest_file])
        if check_res.success and check_res.stdout.strip():
            try:
                rem_data = json.loads(check_res.stdout.strip())
                if rem_data.get("hash") == curr_hash:
                    # CACHE HIT!
                    self.runtime_repo.add_cache_entry(runtime_id, "volume", mount.source, curr_hash)
                    return False
            except Exception:
                pass

        # CACHE MISS: transfer data
        await transport.exec(["mkdir", "-p", remote_vol_dir])

        transfer_choice = decide_transfer_mode(source_path, configured_mode=transfer_mode_pref)
        if transfer_choice.value == "tar-stream":
            await TarStreamTransport.push_tar_stream(source_path, remote_vol_dir, transport)
        else:
            await RsyncTransport.push(source_path, remote_vol_dir, transport)

        # Write remote manifest
        manifest_data = {
            "source": str(source_path),
            "hash": curr_hash,
            "mode": mode.value,
            "synced_at": datetime.now(timezone.utc).isoformat(),
        }
        await transport.upload_text(json.dumps(manifest_data), remote_manifest_file)
        self.runtime_repo.add_cache_entry(runtime_id, "volume", mount.source, curr_hash)
        return True

    async def sync_mount_from_runtime(
        self,
        mount: VolumeMount,
        transport: SSHTransport,
    ) -> None:
        """Pull remote volume data back to local server (for OUTPUT or RW volumes)."""
        vol = self.repo.get_by_name_or_id(mount.source)
        source_path = Path(vol.source if vol else mount.source).resolve()
        source_path.mkdir(parents=True, exist_ok=True, mode=0o755)

        remote_vol_dir = f"/content/.cbox/volumes/{mount.source}"
        await RsyncTransport.pull(remote_vol_dir, source_path, transport)
