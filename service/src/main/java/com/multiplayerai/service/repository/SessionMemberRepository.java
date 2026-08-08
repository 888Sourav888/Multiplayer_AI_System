package com.multiplayerai.service.repository;

import com.multiplayerai.service.entity.SessionMemberEntity;
import com.multiplayerai.service.entity.SessionMemberId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.UUID;

@Repository
public interface SessionMemberRepository extends JpaRepository<SessionMemberEntity, SessionMemberId> {
    List<SessionMemberEntity> findByIdSessionId(UUID sessionId);
    List<SessionMemberEntity> findByIdUserId(UUID userId);
    long countByIdSessionId(UUID sessionId);
}
