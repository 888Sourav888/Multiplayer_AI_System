package com.multiplayerai.service.repository;

import com.multiplayerai.service.entity.SessionEntity;
import com.multiplayerai.service.enums.SessionStatus;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.UUID;

@Repository
public interface SessionRepository extends JpaRepository<SessionEntity, UUID> {
    List<SessionEntity> findByOwnerId(UUID ownerId);
    long countByOwnerIdAndStatus(UUID ownerId, SessionStatus status);
}
