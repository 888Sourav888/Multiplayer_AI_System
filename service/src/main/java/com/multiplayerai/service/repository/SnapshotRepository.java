package com.multiplayerai.service.repository;

import com.multiplayerai.service.entity.SnapshotEntity;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

@Repository
public interface SnapshotRepository extends JpaRepository<SnapshotEntity, UUID> {
    List<SnapshotEntity> findBySessionIdOrderByVersionDesc(UUID sessionId);
    Optional<SnapshotEntity> findBySessionIdAndVersion(UUID sessionId, Long version);
}
