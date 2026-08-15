package com.multiplayerai.service.repository;

import com.multiplayerai.service.entity.AiFileChangeEntity;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;
import java.util.UUID;

@Repository
public interface AiFileChangeRepository extends JpaRepository<AiFileChangeEntity, UUID> {
}
