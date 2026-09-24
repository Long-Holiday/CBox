"""Internal scheduler for GPU matching, cache affinity, and runtime reuse."""

from typing import Optional
from sqlalchemy.orm import Session

from cbox.domain.container import ContainerSpec
from cbox.domain.runtime import Runtime
from cbox.storage.repositories import RuntimeRepository


class Scheduler:
    """Calculates affinity scores and selects the optimal runtime for a container."""

    def __init__(self, session: Session):
        self.session = session
        self.repo = RuntimeRepository(session)

    def score_runtime(
        self,
        runtime: Runtime,
        spec: ContainerSpec,
        image_id: str,
    ) -> int:
        """Calculate placement score according to plan.md section 42:
        score = (
            gpu_match * 100
            + image_cache * 30
            + volume_cache * 50
            + idle_runtime * 100
        )
        """
        score = 0

        # GPU match
        if runtime.accelerator in spec.gpu:
            score += 100

        # Image cache hit
        if self.repo.has_cache(runtime.id, "image", image_id, image_id):
            score += 30

        # Volume cache hit
        for mount in spec.mounts:
            if self.repo.has_cache(runtime.id, "volume", mount.source, mount.source):
                score += 50

        # Idle runtime reuse
        if runtime.state.value == "idle":
            score += 100

        return score

    def select_best_runtime(
        self,
        spec: ContainerSpec,
        image_id: str,
        profile: str = "default",
    ) -> Optional[Runtime]:
        """Find the best existing idle runtime matching requirements."""
        all_runtimes = self.repo.list_runtimes()
        idle_runtimes = [r for r in all_runtimes if r.state.value == "idle" and r.profile == profile]

        candidates = []
        for r in idle_runtimes:
            # Must match at least one requested GPU
            if r.accelerator in spec.gpu:
                score = self.score_runtime(r, spec, image_id)
                candidates.append((score, r))

        if not candidates:
            return None

        # Sort by score descending
        candidates.sort(key=lambda x: x[0], reverse=True)
        return candidates[0][1]
